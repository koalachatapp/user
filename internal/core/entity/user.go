package entity

type UserEntity struct {
	Uuid     string `json:"uuid" gorm:"primaryKey;unique"`
	Username string `json:"username" form:"username" gorm:"unique"`
	Name     string `json:"name" form:"name"`
	Email    string `json:"email" form:"email" gorm:"unique"`
	Password string `json:"password" form:"password"`
}
