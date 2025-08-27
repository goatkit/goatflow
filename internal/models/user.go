package models

import (
	"time"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID               uint      `json:"id" db:"id"`
	Login            string    `json:"login" db:"login"`
	Email            string    `json:"email"` // Not in users table, use login as email
	Password         string    `json:"-" db:"pw"` // Never expose in JSON
	Title            string    `json:"title" db:"title"`
	FirstName        string    `json:"first_name" db:"first_name"`
	LastName         string    `json:"last_name" db:"last_name"`
	ValidID          int       `json:"valid_id" db:"valid_id"` // OTRS valid field (1=valid, 2=invalid, 3=invalid-temporarily)
	CreateTime       time.Time `json:"create_time" db:"create_time"`
	CreateBy         int       `json:"create_by" db:"create_by"`
	ChangeTime       time.Time `json:"change_time" db:"change_time"`
	ChangeBy         int       `json:"change_by" db:"change_by"`
	Role             string    `json:"role"` // Admin, Agent, Customer
	TenantID         uint      `json:"tenant_id,omitempty"`
	LastLogin        *time.Time `json:"last_login,omitempty"`
	FailedLoginCount int       `json:"-"`
	LockedUntil      *time.Time `json:"-"`
	Groups           []string  `json:"groups,omitempty"` // Group names for the user
}

// IsActive returns true if the user is active (valid_id = 1 in OTRS)
func (u *User) IsActive() bool {
	return u.ValidID == 1
}

type UserRole string

const (
	RoleAdmin    UserRole = "Admin"
	RoleAgent    UserRole = "Agent" 
	RoleCustomer UserRole = "Customer"
)

func (u *User) SetPassword(password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hashedPassword)
	return nil
}

func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	return err == nil
}

func (u *User) IsLocked() bool {
	if u.LockedUntil == nil {
		return false
	}
	return time.Now().Before(*u.LockedUntil)
}

func (u *User) LockAccount(duration time.Duration) {
	lockTime := time.Now().Add(duration)
	u.LockedUntil = &lockTime
}

func (u *User) UnlockAccount() {
	u.LockedUntil = nil
	u.FailedLoginCount = 0
}

func (u *User) IncrementFailedLogin() {
	u.FailedLoginCount++
	if u.FailedLoginCount >= 5 {
		u.LockAccount(15 * time.Minute)
	}
}

func (u *User) ResetFailedLogin() {
	u.FailedLoginCount = 0
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginResponse struct {
	Token        string    `json:"token"`
	RefreshToken string    `json:"refresh_token"`
	User         *User     `json:"user"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}