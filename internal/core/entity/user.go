package entity

type UserEntity struct {
	Uuid     string `json:"uuid" gorm:"primaryKey;unique"`
	Username string `json:"username" form:"username"`
	Name     string `json:"name" form:"name"`
	Email    string `json:"email" form:"email"`
	Password string `json:"password" form:"password"`
}
