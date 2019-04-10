package dao

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/lib/pq"
	"github.com/pkg/errors"

	"github.com/jinzhu/gorm"
)

var Daos = []interface{}{new(Dao)}

func IsRecordNotFound(err error) bool {
	return gorm.IsRecordNotFoundError(errors.Cause(err))
}

type BeforeCreateOrUpdate interface {
	BeforeCreateOrUpdate() map[string]interface{}
}

type DbStatsCollector interface {
	AddDbStats(c context.Context, since time.Time, queryFmt string, params ...interface{})
}

type Model interface {
	GetID() uuid.UUID
	GenerateID()
	IsIDNil() bool
}

func NewDebug(database string, connectionString string, models ...interface{}) (*Dao, error) {
	d := &Dao{
		database:         database,
		connectionString: connectionString,
		models:           models,
		Debug:            true,
	}
	if err := d.Init(); err != nil {
		return nil, err
	}
	return d, nil
}

func New(database string, connectionString string, models ...interface{}) (*Dao, error) {
	d := &Dao{
		database:         database,
		connectionString: connectionString,
		models:           models,
	}
	if err := d.Init(); err != nil {
		return nil, err
	}
	return d, nil
}

type Dao struct {
	database         string
	connectionString string
	models           []interface{}
	Logger           Logger
	StatsCollector   DbStatsCollector

	Debug bool

	masterGormDb *gorm.DB

	userMsgsByUniqueIndexes map[string]string
}

func (d *Dao) IsRecordNotFound(err error) bool {
	return IsRecordNotFound(err)
}

func (d *Dao) Init() error {

	if d.Logger == nil {
		d.Logger = NewStdoutLogger()
	}
	if d.StatsCollector == nil {
		d.StatsCollector = newStdoutStatsCollector()
	}

	var err error
	if d.masterGormDb, err = d.initDb(); err != nil {
		return err
	}

	return nil
}

func (d *Dao) initDb() (*gorm.DB, error) {
	db, err := gorm.Open(d.database, d.connectionString)
	//d.Logger.Debugf(context.Background(), "connect string: %s", cs)
	if err != nil {
		return nil, err
	}
	gormDb := db
	d.Logger.Infof(context.Background(), "Database connected")
	d.userMsgsByUniqueIndexes = map[string]string{}

	if d.Debug { // TODO
		gormDb.LogMode(true)
		//d.gormDb.SetLogger(gorm.Logger{revel.TRACE})
		gormDb.SetLogger(d)
	}

	gormDb.BlockGlobalUpdate(true)

	for _, model := range d.models {
		if d.Debug {
			fmt.Printf("migrating %T\n", model)
		}
		if err = db.AutoMigrate(model).Error; err != nil {
			return nil, err
		}
	}

	return gormDb, nil
}

func (d *Dao) addUniqueIndex(db *gorm.DB, index, validationMsg string, model Model, columns ...string) error {
	d.userMsgsByUniqueIndexes[index] = validationMsg
	if err := db.Model(model).AddUniqueIndex(index, columns...).Error; err != nil {
		return errors.Wrapf(err, "adding index %s", index)
	}
	return nil
}

func (d *Dao) Clean() error {
	if err := d.masterGormDb.Close(); err != nil {
		d.Logger.Errorf(context.Background(), "Error closing database: %s", err.Error())
	}
	return nil
}

// Print is used to log sql statements (implementation of gorm.logger)
func (d *Dao) Print(v ...interface{}) {
	// unfortunately, we have no context.Context for the original db operation
	var byts bytes.Buffer
	for _, msg := range gorm.LogFormatter(v...) {
		byts.WriteString(strings.TrimSpace(fmt.Sprintf("%v", msg)))
		byts.WriteRune(' ')
	}
	d.Logger.Debugf(context.Background(), byts.String())
}

func (d *Dao) Query(c context.Context) *Query {
	return &Query{
		c:              c,
		gormDb:         d.masterGormDb, // for queries we can use slave later
		logger:         d.Logger,
		statsCollector: d.StatsCollector,
	}
}

func (d *Dao) hookBeforeSaveOrUpdate(c context.Context, m Model, updateCols map[string]interface{}) {
	b, is := m.(BeforeCreateOrUpdate)
	if !is {
		return
	}
	custom := b.BeforeCreateOrUpdate()
	if updateCols != nil && custom != nil {
		for k, v := range custom {
			if _, has := updateCols[k]; has {
				d.Logger.Warningf(c, "overwriting column %s", k)
			}
			updateCols[k] = v
		}
	}
}

func (d *Dao) IncrColumn(c context.Context, model Model, col string, incr float64) error {
	if err := d.assertIDValid(c, model); err != nil {
		return err
	}
	return d.DeprecatedUpdateColumn(c, model, col, gorm.Expr(col+"+?", incr))
}

type ColumnInfo func() (string, interface{})

func (d *Dao) DeprecatedUpdateColumn(c context.Context, model Model, col string, val interface{}) error {
	return d.DeprecatedUpdateColumns(c, model, map[string]interface{}{col: val})
}

func (d *Dao) UpdateColumns(c context.Context, model Model, ci ...ColumnInfo) error {
	vals := map[string]interface{}{}
	for n := range ci {
		k, v := ci[n]()
		vals[k] = v
	}
	return d.DeprecatedUpdateColumns(c, model, vals)
}

func (d *Dao) DeprecatedUpdateColumns(c context.Context, model Model, cols map[string]interface{}) error {
	if err := d.assertIDValid(c, model); err != nil {
		return err
	}
	started := time.Now()
	defer d.StatsCollector.AddDbStats(c, started, "updating %T", model)

	d.hookBeforeSaveOrUpdate(c, model, cols)

	if !d.masterGormDb.HasBlockGlobalUpdate() {
		return errors.New("no global updates allowed")
	}
	q := d.masterGormDb.Model(model).Update(cols)
	if err := q.Error; err != nil {
		return d.extractUniqueMessages(c, err)
	}
	if q.RowsAffected > 1 {
		d.Logger.Criticalf(c, "Expected 1 update, got %d", q.RowsAffected)
		return errors.New("!")
	}
	return nil
}

func (d *Dao) HardDelete(c context.Context, m Model) error {
	if err := d.assertIDValid(c, m); err != nil {
		return err
	}
	started := time.Now()
	defer d.StatsCollector.AddDbStats(c, started, "hard deleting %T", m)

	q := d.masterGormDb.Unscoped().Delete(m)
	if err := q.Error; err != nil {
		return errors.Wrapf(err, "deleting %T", m)
	}
	if q.RowsAffected > 1 {
		d.Logger.Criticalf(c, "Expected 1 update, got %d", q.RowsAffected)
		return errors.New(fmt.Sprintf("Expected 1 update, got %d", q.RowsAffected))
	}
	return nil
}

func (d *Dao) Delete(c context.Context, m Model) error {
	if err := d.assertIDValid(c, m); err != nil {
		return err
	}
	started := time.Now()
	defer d.StatsCollector.AddDbStats(c, started, "deleting %T", m)

	// Not using gorm's Delete, because we must update updated_at for stitchdata:
	return d.DeprecatedUpdateColumns(c, m, map[string]interface{}{
		"deleted_at": time.Now(),
		"updated_at": time.Now(),
	})
}

// assertIDValid must be called on every update/delete that must be executed on only row. Otherwise if ID
// is a zero value GORM executes a global UPDATE!!!!!!!
func (d *Dao) assertIDValid(c context.Context, m Model) error {
	if m.GetID() == uuid.Nil {
		return errors.New("id nil")
	}
	return nil
}

func (d *Dao) extractUniqueMessages(c context.Context, err error) error {
	if err == nil {
		d.Logger.Warningf(c, "err nil")
		return nil
	}
	if pgErr, is := err.(*pq.Error); is {
		if pgErr.Code == "23505" {
			err = errors.Wrapf(err, "constraint: %s", pgErr.Constraint)
			if msg, found := d.userMsgsByUniqueIndexes[pgErr.Constraint]; found {
				return errors.Wrap(err, msg)
			}
		}
	}
	return errors.Wrap(err, "extracting unique messages")
}

func (d *Dao) CreateOrUpdateColumns(c context.Context, m Model, ci ...ColumnInfo) error {
	if m == nil {
		return errors.Errorf("nil model")
	}
	if m.IsIDNil() {
		return d.Create(c, m)
	}
	return d.UpdateColumns(c, m, ci...)
}

func (d *Dao) CreateOrUpdate(c context.Context, m Model) error {
	if m == nil {
		return errors.New("nil model")
	}
	if m.IsIDNil() {
		return d.Create(c, m)
	}
	started := time.Now()
	defer d.StatsCollector.AddDbStats(c, started, "updating %T", m)

	d.hookBeforeSaveOrUpdate(c, m, nil)
	if err := d.masterGormDb.Save(m).Error; err != nil {
		return errors.Wrapf(err, "saving %T", m)
	}
	return nil
}

func (d *Dao) Create(c context.Context, m Model) error {
	if m == nil {
		return errors.New("nil model")
	}
	if !m.IsIDNil() {
		return errors.Errorf("inserting an existing object, %T with %s", m, m.GetID())
	}
	started := time.Now()
	defer d.StatsCollector.AddDbStats(c, started, "creating %T", m)

	m.GenerateID()
	d.hookBeforeSaveOrUpdate(c, m, nil)
	if err := d.masterGormDb.Create(m).Error; err != nil {
		return d.extractUniqueMessages(c, err)
	}
	return nil
}

func (d *Dao) Load(c context.Context, m Model) error {
	if m.IsIDNil() {
		return errors.Errorf("nil id %T", m)
	}
	started := time.Now()
	defer d.StatsCollector.AddDbStats(c, started, "loading %T", m)
	return d.Query(c).Filter("id", "=", m.GetID()).First(m)
}

func (d *Dao) ByStringID(c context.Context, m Model, strID string) error {
	if strID == "" {
		return errors.New(fmt.Sprintf("model=%T, id empty", m))
	}
	id, err := uuid.FromString(strID)
	if err != nil {
		return errors.Wrapf(err, "id=%s", strID)
	}
	return d.ByID(c, m, id)
}

func (d *Dao) ByID(c context.Context, m Model, id uuid.UUID) error {
	if uuid.Nil == id {
		return errors.Errorf("nil id %T", m)
	}
	started := time.Now()
	defer d.StatsCollector.AddDbStats(c, started, "getting %T", m)
	return d.Query(c).Filter("id", "=", id).First(m)
}

func (d *Dao) GetDeletedByID(c context.Context, m Model, id uuid.UUID) error {
	if uuid.Nil == id {
		return errors.Errorf("nil id %T", m)
	}
	started := time.Now()
	defer d.StatsCollector.AddDbStats(c, started, "getting %T", m)

	return d.Query(c).IncludeDeleted().Filter("id", "=", id).First(m)
}
