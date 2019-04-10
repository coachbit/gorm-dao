package models
/* Run 'go generate' to regenerate this file */

var Columns struct {
User struct {
ID string
CreatedAt string
UpdatedAt string
DeletedAt string
Birth string
Email string
FirstName string
LastName string
Phone string
Code string
}
Something struct {
ID string
CreatedAt string
UpdatedAt string
DeletedAt string
UserID string
Code string
}
UserWithHooks struct {
ID string
CreatedAt string
UpdatedAt string
DeletedAt string
FirstName string
LastName string
Name string
}
}
// nolint
func init() {
Columns.User.ID = "id"
Columns.User.CreatedAt = "created_at"
Columns.User.UpdatedAt = "updated_at"
Columns.User.DeletedAt = "deleted_at"
Columns.User.Birth = "birth"
Columns.User.Email = "email"
Columns.User.FirstName = "name"
Columns.User.LastName = "last_name"
Columns.User.Phone = "phone"
Columns.User.Code = "code"
Columns.Something.ID = "id"
Columns.Something.CreatedAt = "created_at"
Columns.Something.UpdatedAt = "updated_at"
Columns.Something.DeletedAt = "deleted_at"
Columns.Something.UserID = "user_id"
Columns.Something.Code = "code"
Columns.UserWithHooks.ID = "id"
Columns.UserWithHooks.CreatedAt = "created_at"
Columns.UserWithHooks.UpdatedAt = "updated_at"
Columns.UserWithHooks.DeletedAt = "deleted_at"
Columns.UserWithHooks.FirstName = "first_name"
Columns.UserWithHooks.LastName = "last_name"
Columns.UserWithHooks.Name = "name"
}
func (m *User) ColumnID() (string, interface{}) { return "id", m.ID }
func (m *User) ColumnCreatedAt() (string, interface{}) { return "created_at", m.CreatedAt }
func (m *User) ColumnUpdatedAt() (string, interface{}) { return "updated_at", m.UpdatedAt }
func (m *User) ColumnDeletedAt() (string, interface{}) { return "deleted_at", m.DeletedAt }
func (m *User) ColumnBirth() (string, interface{}) { return "birth", m.Birth }
func (m *User) ColumnEmail() (string, interface{}) { return "email", m.Email }
func (m *User) ColumnFirstName() (string, interface{}) { return "name", m.FirstName }
func (m *User) ColumnLastName() (string, interface{}) { return "last_name", m.LastName }
func (m *User) ColumnPhone() (string, interface{}) { return "phone", m.Phone }
func (m *User) ColumnCode() (string, interface{}) { return "code", m.Code }
func (m *Something) ColumnID() (string, interface{}) { return "id", m.ID }
func (m *Something) ColumnCreatedAt() (string, interface{}) { return "created_at", m.CreatedAt }
func (m *Something) ColumnUpdatedAt() (string, interface{}) { return "updated_at", m.UpdatedAt }
func (m *Something) ColumnDeletedAt() (string, interface{}) { return "deleted_at", m.DeletedAt }
func (m *Something) ColumnUserID() (string, interface{}) { return "user_id", m.UserID }
func (m *Something) ColumnCode() (string, interface{}) { return "code", m.Code }
func (m *UserWithHooks) ColumnID() (string, interface{}) { return "id", m.ID }
func (m *UserWithHooks) ColumnCreatedAt() (string, interface{}) { return "created_at", m.CreatedAt }
func (m *UserWithHooks) ColumnUpdatedAt() (string, interface{}) { return "updated_at", m.UpdatedAt }
func (m *UserWithHooks) ColumnDeletedAt() (string, interface{}) { return "deleted_at", m.DeletedAt }
func (m *UserWithHooks) ColumnFirstName() (string, interface{}) { return "first_name", m.FirstName }
func (m *UserWithHooks) ColumnLastName() (string, interface{}) { return "last_name", m.LastName }
func (m *UserWithHooks) ColumnName() (string, interface{}) { return "name", m.Name }
