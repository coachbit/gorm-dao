package dao

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/ansel1/merry"
	utils "github.com/coachbit/gorm-dao/dao/daoutils"
	"github.com/coachbit/gorm-dao/dao/stats"
	"github.com/gofrs/uuid"
	"github.com/lib/pq"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	"github.com/jinzhu/gorm"
)

type DbHook int

const (
	BeforeCreate DbHook = iota
	AfterCreate  DbHook = iota
	BeforeUpdate DbHook = iota
	AfterUpdate  DbHook = iota
	AfterDelete  DbHook = iota
)

type HookCtx struct {
	Fields    map[string]interface{}
	AllFields bool
	Vars      map[string]interface{}
	hook      DbHook
}

func (hctx HookCtx) HasField(fld string) bool {
	if hctx.AllFields {
		return true
	}
	_, found := hctx.Fields[fld]
	return found
}

type ListenerFunc func(c context.Context, hook DbHook, m Model, hctx *HookCtx)

type Listener interface {
	DbHook(c context.Context, hook DbHook, hctx *HookCtx)
}

func IsRecordNotFound(err error) bool {
	return gorm.IsRecordNotFoundError(errors.Cause(merry.Unwrap(err)))
}

func IsUniqueConstraintError(err error) bool {
	err = merry.Unwrap(err)
	if pgErr, is := err.(*pq.Error); is {
		return pgErr.Code == "23505"
	}
	return false
}

type DbStatsCollector interface {
	AddStats(c context.Context, since time.Time, queryFmt string, params ...interface{})
}

type Model interface {
	GetID() uuid.UUID
	GenerateID()
	IsIDNil() bool
}

func NewDebug(version string, database string, connectionString string, models ...interface{}) *Dao {
	d := New(version, database, connectionString, models...)
	d.Debug = true
	return d
}

func New(version string, database string, connectionString string, models ...interface{}) *Dao {
	return &Dao{
		version:                 version,
		database:                database,
		connectionString:        connectionString,
		models:                  models,
		userMsgsByUniqueIndexes: map[string]string{},
	}
}

type Dao struct {
	Logger         Logger                `inject:""`
	StatsCollector *stats.StatsCollector `inject:"db_stats"`

	version          string
	database         string
	connectionString string
	models           []interface{}

	Debug bool

	masterGormDb *gorm.DB

	initialStatements       []string
	userMsgsByUniqueIndexes map[string]string

	modelListenersMutex sync.RWMutex
	modelListeners      map[reflect.Type][]ListenerFunc
}

func (d *Dao) DbInfo(c context.Context) (string, error) {
	rows, err := d.Query(c).RawRows(`select uv.a tablename, pg_size_pretty(uv.b) sizepretty from (select tb.tablename a, pg_table_size('public.'||tb.tablename::text) b from pg_tables tb where tb.schemaname ilike 'public' order by 2 desc) uv`)
	if err != nil {
		return "", err
	}
	defer utils.CloseCloser(rows)

	res := ""
	for rows.Next() {
		var table, size string
		if err := rows.Scan(&table, &size); err != nil {
			return "", err
		}
		res += fmt.Sprintf("%30s: %s\n", table, size)
	}
	return res, nil
}

func (d *Dao) IsRecordNotFound(err error) bool {
	return IsRecordNotFound(err)
}

func (d *Dao) Init() error {
	c := context.Background()

	var err error
	if d.masterGormDb, err = d.initDb(c); err != nil {
		return err
	}

	d.modelListeners = map[reflect.Type][]ListenerFunc{}

	return nil
}

func (d *Dao) initDb(c context.Context) (*gorm.DB, error) {
	db, err := gorm.Open(d.database, d.connectionString)
	if err != nil {
		return nil, err
	}
	gormDb := db
	d.Logger.Infof(c, "Database connected")
	d.userMsgsByUniqueIndexes = map[string]string{}

	if d.Debug {
		gormDb.LogMode(true)
		//d.gormDb.SetLogger(gorm.Logger{revel.TRACE})
		gormDb.SetLogger(d)
	}

	gormDb.BlockGlobalUpdate(true)

	dbVersionFile := "./.coachbit_db_version"
	byts, _ := ioutil.ReadFile(dbVersionFile)
	defer func() {
		_ = os.Remove(dbVersionFile)
		if err := ioutil.WriteFile(dbVersionFile, []byte(d.version), 0700); err != nil {
			d.Logger.Errf(c, err, "error saving db version file")
		}
	}()

	for _, stmt := range d.initialStatements {
		rawDb := db.DB()
		rs, err := rawDb.Exec(stmt)
		if err != nil {
			d.Logger.Errorf(c, "Initial statement error: %s", stmt)
			return nil, merry.Wrap(err).Appendf("stmt: %s", stmt)
		}
		d.Logger.Infof(c, "Initial statement OK: %s", stmt)
		d.Logger.Infof(c, "rs: %#v", rs)
	}

	d.Logger.Infof(c, "Initializing models")
	for _, model := range d.models {
		if d.version == string(byts) {
			fmt.Printf("NOT migrating %T (version unchanged)\n", model)
		} else {
			fmt.Printf("migrating %T\n", model)
			if err = db.AutoMigrate(model).Error; err != nil {
				return nil, err
			}
		}
	}

	return gormDb, nil
}

func (d *Dao) AddListener(m Model, lstnr ListenerFunc) {
	ty := reflect.TypeOf(m)
	d.modelListenersMutex.Lock()
	defer d.modelListenersMutex.Unlock()
	d.modelListeners[ty] = append(d.modelListeners[ty], lstnr)
}

func (d *Dao) AddInitialStatements(c context.Context, statement string) {
	d.initialStatements = append(d.initialStatements, statement)
}

func (d *Dao) AddUniqueIndex(c context.Context, index, validationMsg string, model Model, columns ...string) error {
	d.userMsgsByUniqueIndexes[index] = validationMsg
	if err := d.masterGormDb.Model(model).AddUniqueIndex(index, columns...).Error; err != nil {
		if strings.Contains(err.Error(), "relation") && strings.Contains(err.Error(), "already exists") {
			return nil
		}
		return merry.Wrap(err).Appendf("adding index %s", index)
	}
	return nil
}

func (d *Dao) AddIndex(c context.Context, index, validationMsg string, model Model, columns ...string) error {
	d.userMsgsByUniqueIndexes[index] = validationMsg
	if err := d.masterGormDb.Model(model).AddIndex(index, columns...).Error; err != nil {
		if strings.Contains(err.Error(), "relation") && strings.Contains(err.Error(), "already exists") {
			return nil
		}
		return merry.Wrap(err).Appendf("adding index %s", index)
	}
	return nil
}

func (d *Dao) Clean() error {
	c := context.Background()
	if err := d.masterGormDb.Close(); err != nil {
		d.Logger.Errorf(c, "Error closing database: %s", err.Error())
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
	c := context.Background()
	d.Logger.Debugf(c, byts.String())
}

func (d *Dao) Query(c context.Context) *Query {
	return &Query{
		c:              c,
		gormDb:         d.masterGormDb, // for queries we can use slave later
		logger:         d.Logger,
		statsCollector: d.StatsCollector,
	}
}

type ColumnInfo func() (string, interface{})

func (d *Dao) newHookCtx(cols map[string]interface{}, allFields bool) HookCtx {
	return HookCtx{
		Fields:    cols,
		AllFields: allFields,
		Vars:      map[string]interface{}{},
	}
}

func (d *Dao) executeHookListeners(c context.Context, m Model, hook DbHook, hctx *HookCtx) {
	hctx.hook = hook
	if hctx.Fields == nil {
		hctx.Fields = map[string]interface{}{}
	}
	if lstnr, is := m.(Listener); is {
		lstnr.DbHook(c, hook, hctx)
	}
	d.modelListenersMutex.RLock()
	defer d.modelListenersMutex.RUnlock()
	for _, lstnr := range d.modelListeners[reflect.TypeOf(m)] {
		lstnr(c, hook, m, hctx)
	}
}

func (d *Dao) UpdateColumns(c context.Context, model Model, ci ...ColumnInfo) error {
	vals := map[string]interface{}{}
	for n := range ci {
		k, v := ci[n]()
		vals[k] = v
	}
	return d.UpdateColumnValues(c, model, vals)
}

func (d *Dao) UpdateColumnValues(c context.Context, model Model, cols map[string]interface{}) error {
	if err := d.assertIDValid(c, model); err != nil {
		return err
	}
	started := time.Now()
	defer d.StatsCollector.AddStats(c, started, "updating %T", model)

	hctx := d.newHookCtx(cols, false)
	d.executeHookListeners(c, model, BeforeUpdate, &hctx)

	if !d.masterGormDb.HasBlockGlobalUpdate() {
		return merry.New("no global updates allowed")
	}
	q := d.masterGormDb.Model(model).Update(cols)
	if err := q.Error; err != nil {
		return d.extractUniqueMessages(c, err)
	}
	if q.RowsAffected > 1 {
		d.Logger.Criticalf(c, "Expected 1 update, got %d", q.RowsAffected)
		return merry.New("!")
	}
	d.executeHookListeners(c, model, AfterUpdate, &hctx)
	return nil
}

func (d *Dao) Delete(c context.Context, m Model) error {
	if err := d.assertIDValid(c, m); err != nil {
		return err
	}
	started := time.Now()
	defer d.StatsCollector.AddStats(c, started, "hard deleting %T", m)

	hctx := d.newHookCtx(nil, false)

	q := d.masterGormDb.Unscoped().Delete(m)
	if err := q.Error; err != nil {
		return merry.Wrap(err).Appendf("deleting %T", m)
	}
	if q.RowsAffected > 1 {
		d.Logger.Criticalf(c, "Expected 1 update, got %d", q.RowsAffected)
		return merry.New(fmt.Sprintf("Expected 1 update, got %d", q.RowsAffected))
	}
	d.executeHookListeners(c, m, AfterDelete, &hctx)
	return nil
}

// assertIDValid must be called on every update/delete that must be executed on only row. Otherwise if ID
// is a zero value GORM executes a global UPDATE!!!!!!!
func (d *Dao) assertIDValid(c context.Context, m Model) error {
	if m.GetID() == uuid.Nil {
		return merry.New("id nil")
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
			err = merry.Wrap(err).Appendf("constraint: %s", pgErr.Constraint)
			if msg, found := d.userMsgsByUniqueIndexes[pgErr.Constraint]; found {
				return merry.Wrap(err).Appendf(msg)
			}
		}
	}
	return merry.Wrap(err)
}

func (d *Dao) CreateOrUpdateColumns(c context.Context, m Model, ci ...ColumnInfo) error {
	if m == nil {
		return merry.New("nil model")
	}
	if m.IsIDNil() {
		return d.Create(c, m)
	}
	return d.UpdateColumns(c, m, ci...)
}

func (d *Dao) CreateOrUpdate(c context.Context, m Model) error {
	if m == nil {
		return merry.New("nil model")
	}
	if m.IsIDNil() {
		return d.Create(c, m)
	}

	started := time.Now()
	defer d.StatsCollector.AddStats(c, started, "updating %T", m)

	hctx := d.newHookCtx(nil, true)
	d.executeHookListeners(c, m, BeforeUpdate, &hctx)

	if err := d.masterGormDb.Save(m).Error; err != nil {
		return merry.Wrap(err).Appendf("saving %T", m)
	}

	d.executeHookListeners(c, m, AfterUpdate, &hctx)

	return nil
}

func (d *Dao) CreateMulti(c context.Context, modls ...Model) error {
	var grp errgroup.Group
	for n := range modls {
		func(m Model) {
			grp.Go(func() error {
				return d.Create(c, m)
			})
		}(modls[n])
	}
	return grp.Wait()
}

func (d *Dao) Create(c context.Context, m Model) error {
	if m == nil {
		return merry.New("nil model")
	}
	if !m.IsIDNil() {
		return merry.New(fmt.Sprintf("inserting an existing object, %T with %s", m, m.GetID()))
	}
	started := time.Now()
	defer d.StatsCollector.AddStats(c, started, "creating %T", m)

	m.GenerateID()
	hctx := d.newHookCtx(nil, true)
	d.executeHookListeners(c, m, BeforeCreate, &hctx)
	if err := d.masterGormDb.Create(m).Error; err != nil {
		return d.extractUniqueMessages(c, err)
	}
	d.executeHookListeners(c, m, AfterCreate, &hctx)
	return nil
}

func (d *Dao) Load(c context.Context, m Model) error {
	if m.IsIDNil() {
		return merry.New(fmt.Sprintf("nil id %T", m))
	}
	started := time.Now()
	defer d.StatsCollector.AddStats(c, started, "loading %T", m)
	return d.Query(c).Filter("id", "=", m.GetID()).First(m)
}

func (d *Dao) ByStringID(c context.Context, m Model, strID string) error {
	if strID == "" {
		return merry.New(fmt.Sprintf("model=%T, id empty", m))
	}
	id, err := uuid.FromString(strID)
	if err != nil {
		return merry.Wrap(err).Appendf("id=%s", strID)
	}
	return d.ByID(c, m, id)
}

func (d *Dao) Reload(c context.Context, models ...Model) error {
	for n := range models {
		if err := d.ByID(c, models[n], models[n].GetID()); err != nil {
			return err
		}
	}
	return nil
}

func (d *Dao) ByID(c context.Context, m Model, id uuid.UUID) error {
	if uuid.Nil == id {
		return merry.New(fmt.Sprintf("nil id %T", m))
	}
	started := time.Now()
	defer d.StatsCollector.AddStats(c, started, "getting %T", m)
	return d.Query(c).Filter("id", "=", id).First(m)
}

func (d *Dao) GetDeletedByID(c context.Context, m Model, id uuid.UUID) error {
	if uuid.Nil == id {
		return merry.New(fmt.Sprintf("nil id %T", m))
	}
	started := time.Now()
	defer d.StatsCollector.AddStats(c, started, "getting %T", m)

	return d.Query(c).IncludeDeleted().Filter("id", "=", id).First(m)
}
