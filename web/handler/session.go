package handler

import (
	"backer/user"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const (
	sessionNewTemplate = "session_new.html"
)

type sessionHandler struct {
	userService user.Service
}

func NewSessionHandler(userService user.Service) *sessionHandler {
	return &sessionHandler{userService}
}

func (h *sessionHandler) New(c *gin.Context) {
	c.HTML(http.StatusOK, sessionNewTemplate, nil)
}

func (h *sessionHandler) Create(c *gin.Context) {
	var input user.LoginInput
	err := c.ShouldBind(&input)
	if err != nil {
		c.HTML(http.StatusOK, sessionNewTemplate, gin.H{
			"Error": "Invalid input. Please check your email and password.",
		})
		return
	}

	loggedInUser, err := h.userService.Login(input)
	if err != nil {
		c.HTML(http.StatusOK, sessionNewTemplate, gin.H{
			"Error": "Invalid email or password.",
		})
		return
	}

	if loggedInUser.Role != "admin" {
		c.HTML(http.StatusOK, sessionNewTemplate, gin.H{
			"Error": "You don't have permission to access the admin panel.",
		})
		return
	}

	session := sessions.Default(c)
	session.Set("userID", loggedInUser.ID)
	session.Set("userName", loggedInUser.Name)
	if err := session.Save(); err != nil {
		c.HTML(http.StatusOK, sessionNewTemplate, gin.H{
			"Error": "Failed to save session. Please try again.",
		})
		return
	}

	c.Redirect(http.StatusFound, "/users")
}

func (h *sessionHandler) Destroy(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	session.Save()

	c.Redirect(http.StatusFound, "/login")
}
