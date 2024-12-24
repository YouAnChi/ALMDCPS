package models

import (
	"fuzhu_2/config"
	"log"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (u *User) Create() error {
	// 对密码进行加密
	//这行代码的意思是：使用 bcrypt 库对用户的密码进行加密。具体来说，它的作用是将用户输入的密码（u.Password）转换为一个安全的哈希值，以便存储在数据库中。
	/*代码解析：
	  bcrypt.GenerateFromPassword 是一个函数，用于生成密码的哈希值。
	  []byte(u.Password) 将用户的密码转换为字节切片，这是 GenerateFromPassword 函数所需的输入格式。
	  bcrypt.DefaultCost 是一个参数，表示加密的复杂度。这个值越高，加密过程越慢，但安全性也越高。默认值通常是 10。
	  用法：
	  在用户注册或创建账户时，使用这行代码可以确保用户的密码在存储之前被加密，从而提高安全性。即使数据库被攻击，攻击者也无法直接获取用户的明文密码，只能获取到哈希值。
	  总之，这行代码是实现用户密码安全存储的重要步骤。*/
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("密码加密失败: %v", err)
		return err
	}

	// 插入用户数据
	result, err := config.DB.Exec("INSERT INTO users (username, password) VALUES (?, ?)",
		u.Username, string(hashedPassword))
	if err != nil {
		log.Printf("插入用户数据失败: %v", err)
		return err
	}

	id, _ := result.LastInsertId()
	log.Printf("用户创建成功，ID: %d", id)
	return nil
}

func (u *User) Authenticate() bool {
	var hashedPassword string
	err := config.DB.QueryRow("SELECT password FROM users WHERE username = ?",
		u.Username).Scan(&hashedPassword)
	if err != nil {
		log.Printf("查询用户密码失败: %v", err)
		return false
	}

	log.Printf("数据库中的密码哈希: %s", hashedPassword)
	log.Printf("用户输入的密码: %s", u.Password)
	//通过比较用户输入的密码和存储的哈希值来验证密码的正确性。
	err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(u.Password))
	if err != nil {
		log.Printf("密码验证失败: %v", err)
		return false
	}
	return true
}

// 添加创建测试用户的函数
func CreateTestUser() error {
	// 检查用户是否已存在
	var exists bool
	err := config.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE username = ?)", "test").Scan(&exists)
	if err != nil {
		log.Printf("检查用户是否存在失败: %v", err)
		return err
	}

	if !exists {
		log.Printf("创建测试用户 'test'")
		user := &User{
			Username: "test",
			Password: "123456",
		}
		if err := user.Create(); err != nil {
			log.Printf("创建测试用户失败: %v", err)
			return err
		}
		log.Printf("测试用户创建成功")
	} else {
		log.Printf("测试用户已存在")
	}
	return nil
}
