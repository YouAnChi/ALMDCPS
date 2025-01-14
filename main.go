package main

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"fuzhu_2/api"
	"fuzhu_2/config"
	"fuzhu_2/gongju"
	"fuzhu_2/models"
	"fuzhu_2/types"
	"fuzhu_2/utils"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

var processedRows int
var totalRows int

func main() {
	// 初始化数据库连接
	config.InitDB()
	// 在 main 函数结束时，确保数据库连接被关闭。defer 关键字用于延迟执行 config.DB.Close() 这个函数，直到包含它的函数（在这里是 main 函数）返回。这是一个常见的做法，用于确保资源（如数据库连接）在不再需要时被正确释放，避免资源泄漏。
	defer config.DB.Close()

	// 启动文件清理任务
	// 设置文件最大保存时间为24小时，清理间隔为1小时
	utils.StartCleanupScheduler(
		"./uploads",           // 上传目录
		24*time.Hour,         // 文件最大保存时间
		1*time.Hour,          // 清理检查间隔
	)

	// 创建测试用户
	if err := models.CreateTestUser(); err != nil {
		log.Printf("创建测试用户失败: %v", err)
	} else {
		log.Printf("测试用户初始化完成")
	}

	// 初始化Gin引擎
	r := gin.Default()

	// 设置 session 中间件
	/*用途：
	用户身份验证：会话中间件用于跟踪用户的登录状态。当用户成功登录后，可以将用户信息存储在会话中，以便在后续请求中验证用户身份。
	状态管理：通过会话，可以在多个请求之间存储用户的状态信息，例如购物车内容、用户偏好设置等。
	安全性：使用加密的 Cookie 存储会话数据，可以防止会话劫持和伪造攻击。只有使用正确的密钥才能解密和验证会话数据。*/
	store := cookie.NewStore([]byte("secret"))
	r.Use(sessions.Sessions("mysession", store))

	// 添加认证中间件，用于检查用户登录状态。
	auth := func(c *gin.Context) {
		session := sessions.Default(c) // 获取会话
		user := session.Get("user")    // 获取用户信息
		if user == nil {
			c.Redirect(http.StatusFound, "/login") // 如果用户未登录，重定向到登录页面
			c.Abort()                              // 停止处理请求
			return
		}
		c.Next() // 继续处理请求
	}

	// 设置静态文件目录
	r.Static("/web", "./web")
	// 添加 chengshi 目录的静态文件服务
	r.Static("/chengshi", "./chengshi")

	// 设置首页路由
	r.GET("/", func(c *gin.Context) {
		c.File("./web/home.html") // 返回主页
	})

	// 设置登录页面路由
	r.GET("/login", func(c *gin.Context) {
		c.File("./web/login.html") // 返回登录页面
	})

	// 设置功能页面路由（需要登录）
	r.GET("/dashboard", auth, func(c *gin.Context) {
		c.File("./web/index.html") // 返回功能页面
	})

	// 设置登录API
	/*
		- 如果绑定失败（即 `err` 不为 `nil`），则记录错误信息并返回一个 HTTP 400（Bad Request）状态码，表示请求数据无效。响应中包含一条消息，告知用户请求数据无效。
		- 如果绑定成功，则调用 `user.Authenticate()` 方法验证用户输入的密码是否正确。
		- 如果密码验证成功，则设置 session，将用户信息存储在会话中，以便在后续请求中验证用户身份。
		- 最后，返回一个 HTTP 200（OK）状态码，表示登录成功。响应中包含一条消息，告知用户登录成功，并重定向到关于页面。
	*/
	r.POST("/api/login", func(c *gin.Context) {
		var user models.User
		if err := c.ShouldBindJSON(&user); err != nil {
			log.Printf("解析登录请求失败: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"message": "无效的请求数据"})
			return
		}

		if user.Authenticate() {
			// 设置 session
			session := sessions.Default(c) // 获取会话
			session.Set("user", user.Username)
			session.Save()

			c.JSON(http.StatusOK, gin.H{
				"message":  "登录成功",
				"redirect": "/about", // 登录成功后重定向到关于页面
			})
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{
				"message": "用户名或密码错误",
			})
		}
	})

	// 添加登出 API
	r.POST("/api/logout", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Clear()
		session.Save()
		c.JSON(http.StatusOK, gin.H{
			"message": "登出成功",
		})
	})

	// 设置文件上传的路由
	r.POST("/upload", auth, func(c *gin.Context) {
		processedRows = 0 // 重置处理行数
		file, err := c.FormFile("file")
		if err != nil {
			c.String(http.StatusBadRequest, "获取文件失败: %v", err)
			return
		}

		// 检查文件类型
		if filepath.Ext(file.Filename) != ".xlsx" {
			c.String(http.StatusBadRequest, "不支持的文件类型，请上传.xlsx文件")
			return
		}

		// 保存上传的文件
		filePath := fmt.Sprintf("./uploads/%s", file.Filename)
		if err := c.SaveUploadedFile(file, filePath); err != nil {
			c.String(http.StatusInternalServerError, "保存文件失败: %v", err)
			return
		}

		// 处理上传的文件
		processFile(filePath, c)
	})

	// 提供文件下载服务
	r.Static("/uploads", "./uploads")

	// 提供进度查询服务
	r.GET("/progress", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"processedRows": processedRows,
			"totalRows":     totalRows,
			"completed":     processedRows == totalRows,
		})
	})

	// 添加检查登录状态的 API
	r.GET("/api/check-status", func(c *gin.Context) {
		session := sessions.Default(c)
		user := session.Get("user")
		if user == nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"message": "未登录",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message": "已登录",
			"user":    user,
		})
	})

	// 设置关于页面路由
	r.GET("/about", func(c *gin.Context) {
		c.File("./web/about.html")
	})

	// 设置模型分值计算页面路由
	r.GET("/model-score", func(c *gin.Context) {
		c.File("./web/model_score.html")
	})

	// 添加模型分值计算API
	r.POST("/api/model/score", func(c *gin.Context) {
		gongju.CalculateModelScore(c.Writer, c.Request)
	})

	// 添加Excel文件处理API
	r.POST("/api/process-excel", func(c *gin.Context) {
		gongju.ProcessExcelFile(c.Writer, c.Request)
	})

	// 计算ACC分数的路由
	r.POST("/api/calculate-acc", auth, func(c *gin.Context) {
		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": fmt.Sprintf("获取文件失败: %v", err),
			})
			return
		}

		// 检查文件类型
		if filepath.Ext(file.Filename) != ".xlsx" {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "不支持的文件类型，请上传.xlsx文件",
			})
			return
		}

		// 保存上传的文件
		filePath := fmt.Sprintf("./uploads/%s", file.Filename)
		if err := c.SaveUploadedFile(file, filePath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": fmt.Sprintf("保存文件失败: %v", err),
			})
			return
		}

		// 调用gongju包中的CalculateACCScore函数处理文件
		gongju.CalculateACCScore(c.Writer, c.Request)
	})

	// 设置数据分析页面路由
	r.GET("/data_analysis", func(c *gin.Context) {
		c.File("./web/data_analysis.html")
	})

	// 启动服务器，监听所有IP地址
	r.Run("0.0.0.0:8081")
}

func processFile(filePath string, c *gin.Context) {
	// 获取前端传来的 prompt
	prompt := c.PostForm("prompt")
	if prompt == "" {
		c.String(http.StatusBadRequest, "Prompt 不能为空")
		return
	}

	// 初始化日志系统
	logFile, err := utils.InitLogger()
	if err != nil {
		log.Fatalf("初始化日志系统失败: %v", err)
	}
	defer logFile.Close()

	startTime := time.Now()
	log.Printf("程序开始执行，正在打开输入文件 '%s'...", filePath)

	// 初始化Excel处理器
	excelHandler, err := utils.NewExcelHandler(filePath)
	if err != nil {
		log.Printf("❌ 初始化Excel处理器失败: %v", err)
		c.String(http.StatusInternalServerError, "初始化Excel处理器失败")
		return
	}
	defer excelHandler.Close()

	// 读取所有行
	rows, err := excelHandler.GetRows()
	if err != nil {
		log.Printf("❌ 读取工作表失败: %v", err)
		c.String(http.StatusInternalServerError, "读取工作表失败")
		return
	}
	log.Printf("✅ 成功读取输入文件，共有 %d 行数据需要处理", len(rows))
	totalRows = len(rows)

	// 初始化API客户端
	apiClient := api.NewAPIClient("sk-ad297c6e95034aa896725120f452bac1")
	apiClient.SystemPrompt = prompt // 使用前端传的 prompt

	// 配置并发处理参数
	maxWorkers := 4
	resultChan := make(chan types.Result, len(rows))
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, maxWorkers)

	// 并发处理数据
	log.Printf("开始并发处理数据，并发数: %d", maxWorkers)
	for i, row := range rows {
		if len(row) == 0 {
			log.Printf("⚠️ 跳过第 %d 行：空行", i+1)
			continue
		}

		wg.Add(1)
		go func(rowIndex int, input string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			output := apiClient.ProcessText(input)
			resultChan <- types.Result{
				RowIndex: rowIndex,
				Input:    input,
				Output:   output,
			}
			log.Printf("已处理第 %d 行", rowIndex+1)
			processedRows++
		}(i, row[0])
	}

	// 等待所有处理完成
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// 收集并保存结果
	for result := range resultChan {
		excelHandler.WriteResult(result.RowIndex, result.Input, result.Output)
	}

	// 生成带时间戳的输出文件名
	outputFileName := fmt.Sprintf("output_%s.xlsx", time.Now().Format("2006-01-02_15-04-05"))

	// 保存输出文件
	if err := excelHandler.SaveOutput(filepath.Join("./uploads", outputFileName)); err != nil {
		log.Printf("保存文件失败: %v", err)
		c.String(http.StatusInternalServerError, "保存文件失败")
		return
	}

	// 输出统计信息
	log.Printf("✅ 处理完成！")
	log.Printf("总行数: %d", len(rows))
	log.Printf("总耗时: %v", time.Since(startTime))
	log.Printf("结果已保存到 %s", outputFileName)

	// 返回结果给用户
	c.JSON(http.StatusOK, gin.H{
		"message": "处理完成！",
		"file":    outputFileName,
	})
}

// 使用goroutine并发处理数据：
// 最多同时运行16个工作协程
// 使用channel控制并发数量
// 使用WaitGroup等待所有处理完成
