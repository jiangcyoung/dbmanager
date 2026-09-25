package auth

import (
	"database/sql"
	"time"

	"dbmanager/internal/config"
	"dbmanager/internal/model"
	"dbmanager/internal/store"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type UserStore struct{}

var storeInst *UserStore

func GetStore() *UserStore {
	if storeInst == nil {
		storeInst = &UserStore{}
		storeInst.ensureAdmin()
	}
	return storeInst
}

func (s *UserStore) db() *sql.DB {
	return store.Get().DB()
}

func (s *UserStore) ensureAdmin() {
	cfg := config.GetConfig()
	var count int
	_ = s.db().QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin'").Scan(&count)
	if count > 0 {
		return
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(cfg.Admin.Password), bcrypt.DefaultCost)
	_, _ = s.db().Exec(
		`INSERT INTO users (id, username, password_hash, role, status, created_at)
		 VALUES (?, ?, ?, 'admin', 'active', ?)`,
		uuid.New().String(), cfg.Admin.Username, string(hash), time.Now(),
	)
}

func (s *UserStore) scanUser(row *sql.Row) *model.User {
	u := &model.User{}
	var lastLogin sql.NullTime
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.Status, &u.CreatedAt, &lastLogin)
	if err != nil {
		return nil
	}
	if lastLogin.Valid {
		t := lastLogin.Time
		u.LastLoginAt = &t
	}
	return u
}

func (s *UserStore) scanUsers(rows *sql.Rows) []model.User {
	var users []model.User
	for rows.Next() {
		u := model.User{}
		var lastLogin sql.NullTime
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.Status, &u.CreatedAt, &lastLogin); err != nil {
			continue
		}
		if lastLogin.Valid {
			t := lastLogin.Time
			u.LastLoginAt = &t
		}
		users = append(users, u)
	}
	if users == nil {
		users = []model.User{}
	}
	return users
}

const userCols = "id, username, password_hash, role, status, created_at, last_login_at"

func (s *UserStore) GetByUsername(username string) *model.User {
	row := s.db().QueryRow("SELECT "+userCols+" FROM users WHERE username = ?", username)
	return s.scanUser(row)
}

func (s *UserStore) GetByID(id string) *model.User {
	row := s.db().QueryRow("SELECT "+userCols+" FROM users WHERE id = ?", id)
	return s.scanUser(row)
}

func (s *UserStore) All() []model.User {
	rows, err := s.db().Query("SELECT " + userCols + " FROM users ORDER BY created_at")
	if err != nil {
		return []model.User{}
	}
	defer rows.Close()
	return s.scanUsers(rows)
}

func (s *UserStore) Create(u model.User) error {
	_, err := s.db().Exec(
		`INSERT INTO users (id, username, password_hash, role, status, created_at, last_login_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.Username, u.PasswordHash, u.Role, u.Status, u.CreatedAt, u.LastLoginAt,
	)
	return err
}

func (s *UserStore) Update(id string, fn func(*model.User)) error {
	user := s.GetByID(id)
	if user == nil {
		return nil
	}
	fn(user)
	_, err := s.db().Exec(
		`UPDATE users SET username=?, password_hash=?, role=?, status=?, last_login_at=?
		 WHERE id=?`,
		user.Username, user.PasswordHash, user.Role, user.Status, user.LastLoginAt, id,
	)
	return err
}

func (s *UserStore) Delete(id string) error {
	_, err := s.db().Exec("DELETE FROM users WHERE id = ?", id)
	return err
}
