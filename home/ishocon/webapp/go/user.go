package main

import (
	"context"
	"strconv"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/contrib/sessions"
)

var (
	userCache    sync.Map
	historyCache sync.Map
)

// User model
type User struct {
	ID        int
	Name      string
	Email     string
	Password  string
	LastLogin string
}

func authenticate(email string, password string) (User, bool) {
	var u User
	err := db.QueryRow("SELECT * FROM users WHERE email = ? LIMIT 1", email).Scan(&u.ID, &u.Name, &u.Email, &u.Password, &u.LastLogin)
	if err != nil {
		return u, false
	}
	result := password == u.Password
	return u, result
}

func notAuthenticated(session sessions.Session) bool {
	uid := session.Get("uid")
	return !(uid.(int) > 0)
}

func getUser(ctx context.Context, uid int) User {
	if v, ok := userCache.Load(uid); ok {
		return v.(User)
	}

	u := User{}
	r := db.QueryRow("SELECT * FROM users WHERE id = ? LIMIT 1", uid)
	err := r.Scan(&u.ID, &u.Name, &u.Email, &u.Password, &u.LastLogin)
	if err != nil {
		return u
	}

	return u
}

func currentUser(ctx context.Context, session sessions.Session) User {
	v := session.Get("uid")
	if uid, ok := v.(int); ok {
		return getUser(ctx, uid)
	}
	return User{}
}

// BuyingHistory : products which user had bought
func (u *User) BuyingHistory(ctx context.Context) (products []Product) {
	if v, ok := historyCache.Load(u.ID); ok {
		products = v.([]Product)
	}
	return
}

// BuyProduct : buy product
func (u *User) BuyProduct(ctx context.Context, pid int) {
	now := time.Now()

	if v, ok := historyCache.Load(u.ID); ok {
		h := v.([]Product)
		p := getProduct(ctx, pid)
		p.CreatedAt = now.Format("2006-01-02 15:04:05")
		h = append([]Product{p}, h...)
		historyCache.Store(u.ID, h)
	}

	go db.Exec(
		"INSERT INTO histories (product_id, user_id, created_at) VALUES (?, ?, ?)",
		pid, u.ID, now)
}

// CreateComment : create comment to the product
func (u *User) CreateComment(pidStr string, content string) {
	now := time.Now()
	pid, _ := strconv.Atoi(pidStr)

	if v, ok := commentCache.Load(pid); ok {
		cs := v.([]Comment)
		sc := content
		if utf8.RuneCountInString(sc) > 25 {
			sc = string([]rune(sc)[:25]) + "…"
		}
		cs = append([]Comment{{
			ProductID:    pid,
			UserID:       u.ID,
			Content:      content,
			ShortContent: sc,
			CreatedAt:    now.Format("2006-01-02 15:04:05"),
		}}, cs...)
		commentCache.Store(pid, cs)
	}

	page := pages[pid]
	pageCache.Delete(page)

	db.Exec(
		"INSERT INTO comments (product_id, user_id, content, created_at) VALUES (?, ?, ?, ?)",
		pidStr, u.ID, content, now)
}

func (u *User) UpdateLastLogin() {
	now := time.Now()

	if v, ok := userCache.Load(u.ID); ok {
		user := v.(User)
		user.LastLogin = now.Format("2006-01-02 15:04:05")
		userCache.Store(u.ID, user)
	}

	db.Exec("UPDATE users SET last_login = ? WHERE id = ?", now, u.ID)
}
