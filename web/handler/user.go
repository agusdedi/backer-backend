package handler

import (
	"backer/user"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

const (
	userIndexTemplate = "user_index.html"
	userEditTemplate  = "user_edit.html"
	userNewTemplate   = "user_new.html"
	errorTemplate     = "error.html"
)

type userHandler struct {
	userService user.Service
}

func NewUserHandler(userService user.Service) *userHandler {
	return &userHandler{userService}
}

func (h *userHandler) Index(c *gin.Context) {
	users, err := h.userService.GetAllUsers()
	if err != nil {
		c.HTML(http.StatusInternalServerError, errorTemplate, nil)
		return
	}

	c.HTML(http.StatusOK, userIndexTemplate, gin.H{"users": users})
}

func (h *userHandler) New(c *gin.Context) {
	c.HTML(http.StatusOK, userNewTemplate, gin.H{
		"Input":  user.FormCreateUserInput{},
		"Errors": nil,
	})
}

func (h *userHandler) Create(c *gin.Context) {
	var input user.FormCreateUserInput
	err := c.ShouldBind(&input)
	if err != nil {
		c.HTML(http.StatusOK, userNewTemplate, gin.H{
			"Input":  input,
			"Errors": formatUserFormErrors(err),
		})
		return
	}

	registerInput := user.RegisterUserInput{}
	registerInput.Name = input.Name
	registerInput.Email = input.Email
	registerInput.Occupation = input.Occupation
	registerInput.Password = input.Password

	_, err = h.userService.RegisterUser(registerInput)
	if err != nil {
		c.HTML(http.StatusOK, userNewTemplate, gin.H{
			"Input":  input,
			"Errors": []string{"Failed to create user. The email may already be in use."},
		})
		return
	}

	c.Redirect(http.StatusFound, "/users")
}

// formatUserFormErrors converts raw validator errors into friendly,
// human-readable messages for the admin form.
func formatUserFormErrors(err error) []string {
	var messages []string

	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		return []string{"Invalid input. Please check the form and try again."}
	}

	for _, fieldErr := range validationErrors {
		switch fieldErr.Tag() {
		case "required":
			messages = append(messages, fieldErr.Field()+" is required.")
		case "email":
			messages = append(messages, "Please enter a valid email address.")
		default:
			messages = append(messages, fieldErr.Field()+" is invalid.")
		}
	}

	return messages
}

func (h *userHandler) Edit(c *gin.Context) {
	idParam := c.Param("id")
	id, _ := strconv.Atoi(idParam)

	registeredUser, err := h.userService.GetUserByID(id)
	if err != nil {
		c.HTML(http.StatusInternalServerError, errorTemplate, nil)
		return
	}

	input := user.FormUpdateUserInput{}
	input.ID = registeredUser.ID
	input.Name = registeredUser.Name
	input.Email = registeredUser.Email
	input.Occupation = registeredUser.Occupation

	c.HTML(http.StatusOK, userEditTemplate, gin.H{
		"Input":  input,
		"Errors": nil,
	})
}

func (h *userHandler) Update(c *gin.Context) {
	idParam := c.Param("id")
	id, _ := strconv.Atoi(idParam)

	var input user.FormUpdateUserInput
	input.ID = id

	err := c.ShouldBind(&input)
	if err != nil {
		c.HTML(http.StatusOK, userEditTemplate, gin.H{
			"Input":  input,
			"Errors": formatUserFormErrors(err),
		})
		return
	}

	_, err = h.userService.UpdateUser(input)
	if err != nil {
		c.HTML(http.StatusOK, userEditTemplate, gin.H{
			"Input":  input,
			"Errors": []string{"Failed to update user. Please try again."},
		})
		return
	}

	c.Redirect(http.StatusFound, "/users")
}
