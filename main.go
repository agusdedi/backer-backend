package main

import (
	"backer/auth"
	"backer/campaign"
	"backer/config"
	"backer/handler"
	"backer/helper"
	"backer/payment"
	"backer/transaction"
	"backer/user"
	webHandler "backer/web/handler"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/multitemplate"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const (
	usersPath        = "/users"
	campaignsPath    = "/campaigns"
	transactionsPath = "/transactions"
)

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: failed to load .env file:", err)
	}

	// Load config from .env or environment variables
	config.LoadConfig()

	// Database connection (reads from .env via config package)
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		config.AppConfig.DBUser,
		config.AppConfig.DBPassword,
		config.AppConfig.DBHost,
		config.AppConfig.DBPort,
		config.AppConfig.DBName,
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Repository
	userRepository := user.NewRepository(db)
	campaignRepository := campaign.NewRepository(db)
	transactionRepository := transaction.NewRepository(db)

	// Service
	authService := auth.NewService()
	userService := user.NewService(userRepository)
	campaignService := campaign.NewService(campaignRepository)
	paymentService := payment.NewService()
	transactionService := transaction.NewService(transactionRepository, campaignRepository, paymentService)

	// Handler
	userHandler := handler.NewUserHandler(userService, authService)
	campaignHandler := handler.NewCampaignHandler(campaignService)
	transactionHandler := handler.NewTransactionHandler(transactionService)

	// Web Handler
	userWebHandler := webHandler.NewUserHandler(userService)
	campaignWebHandler := webHandler.NewCampaignHandler(campaignService, userService)
	transactionWebHandler := webHandler.NewTransactionHandler(transactionService)
	sessionWebHandler := webHandler.NewSessionHandler(userService)

	// Router
	router := gin.Default()

	allowedOrigins := []string{"http://localhost:3000"}
	if extraOrigin := os.Getenv("EXTRA_CORS_ORIGIN"); extraOrigin != "" {
		allowedOrigins = append(allowedOrigins, extraOrigin)
	}

	// CORS middleware
	router.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Session middleware
	cookiesStore := cookie.NewStore([]byte(config.AppConfig.SessionSecretKey))
	router.Use(sessions.Sessions("backer", cookiesStore))

	// Load HTML templates
	router.HTMLRender = loadTemplates("./web/templates")

	router.Static("/images", "./images")
	router.Static("/css", "./web/assets/css/")
	router.Static("/js", "./web/assets/js/")
	router.Static("/webfonts", "./web/assets/webfonts/")

	// API routes
	api := router.Group("/api/v1")

	// User routes
	api.POST(usersPath, userHandler.RegisterUser)
	api.POST("/sessions", userHandler.Login)
	api.POST("/email_checkers", userHandler.CheckEmailAvailability)
	api.POST("/avatars", authMiddleware(authService, userService), userHandler.UploadAvatar)
	api.GET("/users/fetch", authMiddleware(authService, userService), userHandler.FetchUser)

	// Campaign routes
	api.GET(campaignsPath, campaignHandler.GetCampaigns)
	api.GET(campaignsPath+"/:id", campaignHandler.GetCampaign)
	api.POST(campaignsPath, authMiddleware(authService, userService), campaignHandler.CreateCampaign)
	api.PUT(campaignsPath+"/:id", authMiddleware(authService, userService), campaignHandler.UpdateCampaign)
	api.POST("/campaign-images", authMiddleware(authService, userService), campaignHandler.UploadImage)

	// Transaction routes
	api.GET(campaignsPath+"/:id/transactions", authMiddleware(authService, userService), transactionHandler.GetCampaignTransactions)
	api.GET(transactionsPath, authMiddleware(authService, userService), transactionHandler.GetUserTransactions)
	api.POST(transactionsPath, authMiddleware(authService, userService), transactionHandler.CreateTransaction)
	api.POST(transactionsPath+"/notification", transactionHandler.GetNotification)

	// Admin CMS web routes for users
	router.GET(usersPath, authAdminMiddleware(), userWebHandler.Index)
	router.GET(usersPath+"/new", authAdminMiddleware(), userWebHandler.New)
	router.POST(usersPath, authAdminMiddleware(), userWebHandler.Create)
	router.GET(usersPath+"/edit/:id", authAdminMiddleware(), userWebHandler.Edit)
	router.POST(usersPath+"/update/:id", authAdminMiddleware(), userWebHandler.Update)
	router.GET(usersPath+"/avatar/:id", authAdminMiddleware(), userWebHandler.Avatar)
	router.POST(usersPath+"/avatar/:id", authAdminMiddleware(), userWebHandler.UpdateAvatar)

	// Admin CMS web routes for campaigns
	router.GET(campaignsPath, authAdminMiddleware(), campaignWebHandler.Index)
	router.GET(campaignsPath+"/new", authAdminMiddleware(), campaignWebHandler.New)
	router.POST(campaignsPath, authAdminMiddleware(), campaignWebHandler.Create)
	router.GET(campaignsPath+"/image/:id", authAdminMiddleware(), campaignWebHandler.NewImage)
	router.POST(campaignsPath+"/image/:id", authAdminMiddleware(), campaignWebHandler.CreateImage)
	router.GET(campaignsPath+"/edit/:id", authAdminMiddleware(), campaignWebHandler.Edit)
	router.POST(campaignsPath+"/update/:id", authAdminMiddleware(), campaignWebHandler.Update)
	router.GET(campaignsPath+"/show/:id", authAdminMiddleware(), campaignWebHandler.Show)

	// Admin CMS web routes for transactions
	router.GET(transactionsPath, authAdminMiddleware(), transactionWebHandler.Index)

	// Admin CMS web routes for session
	router.GET("/login", sessionWebHandler.New)
	router.POST("/session", sessionWebHandler.Create)
	router.GET("/logout", sessionWebHandler.Destroy)

	// Start the server
	router.Run(":8080")
}

func authMiddleware(authService auth.Service, userService user.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if !strings.Contains(authHeader, "Bearer") {
			response := helper.APIResponse("Unauthorized", http.StatusUnauthorized, "error", nil)
			c.AbortWithStatusJSON(http.StatusUnauthorized, response)
			return
		}

		tokenString := ""
		arrayToken := strings.Split(authHeader, " ")
		if len(arrayToken) == 2 {
			tokenString = arrayToken[1]
		}

		token, err := authService.ValidateToken(tokenString)
		if err != nil {
			response := helper.APIResponse("Unauthorized", http.StatusUnauthorized, "error", nil)
			c.AbortWithStatusJSON(http.StatusUnauthorized, response)
			return
		}

		claim, ok := token.Claims.(jwt.MapClaims)
		if !ok || !token.Valid {
			response := helper.APIResponse("Unauthorized", http.StatusUnauthorized, "error", nil)
			c.AbortWithStatusJSON(http.StatusUnauthorized, response)
			return
		}

		userID := int(claim["user_id"].(float64))

		currentUser, err := userService.GetUserByID(userID)
		if err != nil {
			response := helper.APIResponse("Unauthorized", http.StatusUnauthorized, "error", nil)
			c.AbortWithStatusJSON(http.StatusUnauthorized, response)
			return
		}

		c.Set("currentUser", currentUser)
	}
}

func authAdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)

		userIDSession := session.Get("userID")
		if userIDSession == nil {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		c.Next()
	}
}

func loadTemplates(templatesDir string) multitemplate.Renderer {
	r := multitemplate.NewRenderer()

	layouts, err := filepath.Glob(templatesDir + "/layouts/*")
	if err != nil {
		panic(err.Error())
	}

	includes, err := filepath.Glob(templatesDir + "/**/*")
	if err != nil {
		panic(err.Error())
	}

	for _, include := range includes {
		layoutCopy := make([]string, len(layouts))
		copy(layoutCopy, layouts)
		files := append(layoutCopy, include)
		r.AddFromFiles(filepath.Base(include), files...)
	}
	return r
}
