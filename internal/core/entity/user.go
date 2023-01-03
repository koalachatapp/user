package entity

type UserEntity struct {
	Email    string `json:"email" form:"email" gorm:"unique"`
	Name     string `json:"name" form:"name"`
	Password string `json:"password" form:"password"`
	Username string `json:"username" form:"username" gorm:"unique"`
	Uuid     string `json:"uuid" gorm:"primaryKey;unique"`
}
