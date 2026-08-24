package handler

import (
	"backer/auth"
	"backer/helper"
	"backer/storage"
	"backer/user"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type userHandler struct {
	userService user.Service
	authService auth.Service
}

func NewUserHandler(userService user.Service, authService auth.Service) *userHandler {
	return &userHandler{userService, authService}
}

func (h *userHandler) RegisterUser(c *gin.Context) {
	var input user.RegisterUserInput

	err := c.ShouldBindJSON(&input)
	if err != nil {
		validationErrors := helper.FormatValidationError(err)
		errorMessage := gin.H{"errors": validationErrors}

		response := helper.APIResponse(helper.MsgInvalidInput, http.StatusUnprocessableEntity, "error", errorMessage)
		c.JSON(http.StatusUnprocessableEntity, response)
		return
	}

	newUser, err := h.userService.RegisterUser(input)
	if err != nil {
		errorMessage := gin.H{"errors": err.Error()}

		if errors.Is(err, user.ErrEmailAlreadyRegistered) {
			response := helper.APIResponse(helper.MsgEmailAlreadyRegistered, http.StatusConflict, "error", errorMessage)
			c.JSON(http.StatusConflict, response)
			return
		}

		response := helper.APIResponse(helper.MsgRegistrationFailed, http.StatusBadRequest, "error", errorMessage)
		c.JSON(http.StatusBadRequest, response)
		return
	}

	token, err := h.authService.GenerateToken(newUser.ID)
	if err != nil {
		response := helper.APIResponse(helper.MsgFailedToGenerateToken, http.StatusInternalServerError, "error", nil)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	formatter := user.FormatUser(newUser, token)

	response := helper.APIResponse(helper.MsgAccountRegisteredSuccessfully, http.StatusCreated, "success", formatter)
	c.JSON(http.StatusCreated, response)
}

func (h *userHandler) Login(c *gin.Context) {
	var input user.LoginInput

	err := c.ShouldBindJSON(&input)
	if err != nil {
		validationErrors := helper.FormatValidationError(err)
		errorMessage := gin.H{"errors": validationErrors}

		response := helper.APIResponse(helper.MsgInvalidInput, http.StatusUnprocessableEntity, "error", errorMessage)
		c.JSON(http.StatusUnprocessableEntity, response)
		return
	}

	loggedinUser, err := h.userService.Login(input)
	if err != nil {
		errorMessage := gin.H{"errors": err.Error()}

		response := helper.APIResponse(helper.MsgInvalidEmailOrPassword, http.StatusUnauthorized, "error", errorMessage)
		c.JSON(http.StatusUnauthorized, response)
		return
	}

	token, err := h.authService.GenerateToken(loggedinUser.ID)
	if err != nil {
		response := helper.APIResponse(helper.MsgFailedToGenerateToken, http.StatusInternalServerError, "error", nil)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	formatter := user.FormatUser(loggedinUser, token)

	response := helper.APIResponse(helper.MsgSuccessfullyLoggedIn, http.StatusOK, "success", formatter)
	c.JSON(http.StatusOK, response)
}

func (h *userHandler) CheckEmailAvailability(c *gin.Context) {
	var input user.CheckEmailInput

	err := c.ShouldBindJSON(&input)
	if err != nil {
		validationErrors := helper.FormatValidationError(err)
		errorMessage := gin.H{"errors": validationErrors}

		response := helper.APIResponse(helper.MsgEmailValidationFailed, http.StatusUnprocessableEntity, "error", errorMessage)
		c.JSON(http.StatusUnprocessableEntity, response)
		return
	}

	isEmailAvailable, err := h.userService.IsEmailAvailable(input)
	if err != nil {
		errorMessage := gin.H{"errors": helper.MsgUnableToCheckEmailAvailability}
		response := helper.APIResponse(helper.MsgEmailCheckFailed, http.StatusInternalServerError, "error", errorMessage)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	data := gin.H{
		"is_available": isEmailAvailable,
	}

	metaMessage := helper.MsgEmailAlreadyRegisteredCheck
	if isEmailAvailable {
		metaMessage = helper.MsgEmailAvailable
	}

	response := helper.APIResponse(metaMessage, http.StatusOK, "success", data)
	c.JSON(http.StatusOK, response)
}

func (h *userHandler) UploadAvatar(c *gin.Context) {
	file, err := c.FormFile("avatar")
	if err != nil {
		data := gin.H{"is_uploaded": false}
		response := helper.APIResponse(helper.MsgNoAvatarFileProvided, http.StatusBadRequest, "error", data)
		c.JSON(http.StatusBadRequest, response)
		return
	}

	currentUser := c.MustGet("currentUser").(user.User)
	userID := currentUser.ID

	path := fmt.Sprintf("images/avatars/%d-%s", userID, file.Filename)

	src, err := file.Open()
	if err != nil {
		data := gin.H{"is_uploaded": false}
		response := helper.APIResponse(helper.MsgFailedToSaveAvatarFile, http.StatusInternalServerError, "error", data)
		c.JSON(http.StatusInternalServerError, response)
		return
	}
	defer src.Close()

	err = storage.UploadFile(c.Request.Context(), src, path, file.Header.Get("Content-Type"))
	if err != nil {
		data := gin.H{"is_uploaded": false}
		response := helper.APIResponse(helper.MsgFailedToSaveAvatarFile, http.StatusInternalServerError, "error", data)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	_, err = h.userService.SaveAvatar(userID, path)
	if err != nil {
		data := gin.H{"is_uploaded": false}
		response := helper.APIResponse(helper.MsgFailedToUpdateAvatar, http.StatusInternalServerError, "error", data)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	data := gin.H{"is_uploaded": true}
	response := helper.APIResponse(helper.MsgAvatarUploadedSuccessfully, http.StatusOK, "success", data)
	c.JSON(http.StatusOK, response)
}

func (h *userHandler) FetchUser(c *gin.Context) {
	currentUser := c.MustGet("currentUser").(user.User)
	formatter := user.FormatUser(currentUser, "")

	response := helper.APIResponse(helper.MsgUserDataRetrievedSuccessfully, http.StatusOK, "success", formatter)
	c.JSON(http.StatusOK, response)
}
