package handler

import (
	"backer/storage"
	"backer/user"
	"fmt"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

const (
	userIndexTemplate  = "user_index.html"
	userEditTemplate   = "user_edit.html"
	userNewTemplate    = "user_new.html"
	errorTemplate      = "error.html"
	userAvatarTemplate = "user_avatar.html"
	usersRedirectPath  = "/users"

	avatarUploadDir     = "images/avatars"
	maxAvatarUploadSize = 2 << 20 // 2 MB
)

// allowedAvatarExtensions whitelists which file extensions are accepted
// for avatar uploads.
var allowedAvatarExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
}

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

	c.Redirect(http.StatusFound, usersRedirectPath)
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

	c.Redirect(http.StatusFound, usersRedirectPath)
}

func (h *userHandler) Avatar(c *gin.Context) {
	idParam := c.Param("id")
	id, _ := strconv.Atoi(idParam)

	c.HTML(http.StatusOK, userAvatarTemplate, gin.H{"ID": id})
}

func (h *userHandler) UpdateAvatar(c *gin.Context) {
	idParam := c.Param("id")
	id, _ := strconv.Atoi(idParam)

	file, err := c.FormFile("avatar")
	if err != nil {
		c.HTML(http.StatusOK, userAvatarTemplate, gin.H{
			"ID":     id,
			"Errors": []string{"Failed to upload avatar. Please try again."},
		})
		return
	}

	if err := validateAvatarFile(file); err != nil {
		c.HTML(http.StatusOK, userAvatarTemplate, gin.H{
			"ID":     id,
			"Errors": []string{err.Error()},
		})
		return
	}

	filename := fmt.Sprintf("%d-%s", id, sanitizeAvatarFilename(file.Filename))
	path := filepath.Join(avatarUploadDir, filename)

	src, err := file.Open()
	if err != nil {
		c.HTML(http.StatusOK, userAvatarTemplate, gin.H{
			"ID":     id,
			"Errors": []string{"Failed to save avatar file. Please try again."},
		})
		return
	}
	defer src.Close()

	if err := storage.UploadFile(c.Request.Context(), src, path, file.Header.Get("Content-Type")); err != nil {
		c.HTML(http.StatusOK, userAvatarTemplate, gin.H{
			"ID":     id,
			"Errors": []string{"Failed to save avatar file. Please try again."},
		})
		return
	}

	if _, err := h.userService.SaveAvatar(id, path); err != nil {
		c.HTML(http.StatusOK, userAvatarTemplate, gin.H{
			"ID":     id,
			"Errors": []string{"Failed to update avatar. Please try again."},
		})
		return
	}

	c.Redirect(http.StatusFound, usersRedirectPath)
}

// sanitizeAvatarFilename converts a user-supplied filename into a safe
// filename that can be used on the server's filesystem.
func sanitizeAvatarFilename(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	if !allowedAvatarExtensions[ext] {
		ext = ""
	}
	base := strings.TrimSuffix(name, filepath.Ext(name))
	base = strings.Join(strings.Fields(base), "-")

	var b strings.Builder
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		}
	}

	sanitized := b.String()
	if sanitized == "" {
		sanitized = "avatar"
	}

	return sanitized + ext
}

// validateAvatarFile checks the uploaded file's size and extension before
// it's written to disk or handed to the service layer.
func validateAvatarFile(file *multipart.FileHeader) error {
	if file.Size > maxAvatarUploadSize {
		return fmt.Errorf("file is too large. Maximum size is %d MB", maxAvatarUploadSize/(1<<20))
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedAvatarExtensions[ext] {
		return fmt.Errorf("unsupported file type. Allowed types: jpg, jpeg, png")
	}

	return nil
}
