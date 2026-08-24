package handler

import (
	"backer/campaign"
	"backer/helper"
	"backer/storage"
	"backer/user"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type campaignHandler struct {
	service campaign.Service
}

func NewCampaignHandler(service campaign.Service) *campaignHandler {
	return &campaignHandler{service}
}

func (h *campaignHandler) GetCampaigns(c *gin.Context) {
	userID, _ := strconv.Atoi(c.Query("user_id"))

	campaigns, err := h.service.GetCampaigns(userID)
	if err != nil {
		response := helper.APIResponse(helper.MsgFailedToGetCampaigns, http.StatusInternalServerError, "error", nil)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	response := helper.APIResponse(helper.MsgListOfCampaignsRetrieved, http.StatusOK, "success", campaign.FormatCampaigns(campaigns))
	c.JSON(http.StatusOK, response)
}

func (h *campaignHandler) GetCampaign(c *gin.Context) {
	var input campaign.GetCampaignDetailInput

	err := c.ShouldBindUri(&input)
	if err != nil {
		response := helper.APIResponse(helper.MsgInvalidCampaignID, http.StatusBadRequest, "error", nil)
		c.JSON(http.StatusBadRequest, response)
		return
	}

	campaignDetail, err := h.service.GetCampaignByID(input)
	if err != nil {
		errorMessage := gin.H{"errors": err.Error()}

		response := helper.APIResponse(helper.MsgCampaignNotFound, http.StatusNotFound, "error", errorMessage)
		c.JSON(http.StatusNotFound, response)
		return
	}

	response := helper.APIResponse(helper.MsgCampaignDetailRetrieved, http.StatusOK, "success", campaign.FormatCampaignDetail(campaignDetail))
	c.JSON(http.StatusOK, response)
}

func (h *campaignHandler) CreateCampaign(c *gin.Context) {
	var input campaign.CreateCampaignInput

	err := c.ShouldBindJSON(&input)
	if err != nil {
		validationErrors := helper.FormatValidationError(err)
		errorMessage := gin.H{"errors": validationErrors}

		response := helper.APIResponse(helper.MsgInvalidInput, http.StatusUnprocessableEntity, "error", errorMessage)
		c.JSON(http.StatusUnprocessableEntity, response)
		return
	}

	currentUser := c.MustGet("currentUser").(user.User)
	input.User = currentUser

	newCampaign, err := h.service.CreateCampaign(input)
	if err != nil {
		errorMessage := gin.H{"errors": err.Error()}

		response := helper.APIResponse(helper.MsgFailedToCreateCampaign, http.StatusInternalServerError, "error", errorMessage)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	response := helper.APIResponse(helper.MsgCampaignCreatedSuccessfully, http.StatusCreated, "success", campaign.FormatCampaign(newCampaign))
	c.JSON(http.StatusCreated, response)
}

func (h *campaignHandler) UpdateCampaign(c *gin.Context) {
	var inputID campaign.GetCampaignDetailInput

	err := c.ShouldBindUri(&inputID)
	if err != nil {
		validationErrors := helper.FormatValidationError(err)
		errorMessage := gin.H{"errors": validationErrors}

		response := helper.APIResponse(helper.MsgInvalidCampaignID, http.StatusBadRequest, "error", errorMessage)
		c.JSON(http.StatusBadRequest, response)
		return
	}

	var inputData campaign.CreateCampaignInput

	err = c.ShouldBindJSON(&inputData)
	if err != nil {
		validationErrors := helper.FormatValidationError(err)
		errorMessage := gin.H{"errors": validationErrors}

		response := helper.APIResponse(helper.MsgInvalidInput, http.StatusUnprocessableEntity, "error", errorMessage)
		c.JSON(http.StatusUnprocessableEntity, response)
		return
	}

	currentUser := c.MustGet("currentUser").(user.User)
	inputData.User = currentUser

	updatedCampaign, err := h.service.UpdateCampaign(inputID, inputData)
	if err != nil {
		errorMessage := gin.H{"errors": err.Error()}

		if strings.Contains(err.Error(), "not found") {
			response := helper.APIResponse(helper.MsgCampaignNotFound, http.StatusNotFound, "error", errorMessage)
			c.JSON(http.StatusNotFound, response)
			return
		}

		if strings.Contains(err.Error(), "unauthorized") || strings.Contains(err.Error(), "not authorized") {
			response := helper.APIResponse(helper.MsgNotAuthorizedToUpdateCampaign, http.StatusForbidden, "error", errorMessage)
			c.JSON(http.StatusForbidden, response)
			return
		}

		response := helper.APIResponse(helper.MsgFailedToUpdateCampaign, http.StatusInternalServerError, "error", errorMessage)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	response := helper.APIResponse(helper.MsgCampaignUpdatedSuccessfully, http.StatusOK, "success", campaign.FormatCampaign(updatedCampaign))
	c.JSON(http.StatusOK, response)
}

func (h *campaignHandler) UploadImage(c *gin.Context) {
	var input campaign.CreateCampaignImageInput

	err := c.ShouldBind(&input)
	if err != nil {
		validationErrors := helper.FormatValidationError(err)
		errorMessage := gin.H{"errors": validationErrors}

		response := helper.APIResponse(helper.MsgInvalidInput, http.StatusUnprocessableEntity, "error", errorMessage)
		c.JSON(http.StatusUnprocessableEntity, response)
		return
	}

	currentUser := c.MustGet("currentUser").(user.User)
	input.User = currentUser
	userID := currentUser.ID

	file, err := c.FormFile("file")
	if err != nil {
		data := gin.H{"is_uploaded": false}
		response := helper.APIResponse(helper.MsgNoFileUploaded, http.StatusBadRequest, "error", data)
		c.JSON(http.StatusBadRequest, response)
		return
	}

	allowedExtensions := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedExtensions[ext] {
		data := gin.H{"is_uploaded": false}
		response := helper.APIResponse(helper.MsgInvalidFileType, http.StatusBadRequest, "error", data)
		c.JSON(http.StatusBadRequest, response)
		return
	}

	maxSize := int64(2 << 20)
	if file.Size > maxSize {
		data := gin.H{"is_uploaded": false}
		response := helper.APIResponse(helper.MsgFileTooLarge, http.StatusBadRequest, "error", data)
		c.JSON(http.StatusBadRequest, response)
		return
	}

	err = h.service.ValidateCampaignOwnership(input.CampaignID, userID)
	if err != nil {
		errorMessage := gin.H{"errors": err.Error(), "is_uploaded": false}

		if strings.Contains(err.Error(), "not found") {
			response := helper.APIResponse(helper.MsgCampaignNotFound, http.StatusNotFound, "error", errorMessage)
			c.JSON(http.StatusNotFound, response)
			return
		}

		if strings.Contains(err.Error(), "not authorized") {
			response := helper.APIResponse(helper.MsgNotAuthorizedToUploadImage, http.StatusForbidden, "error", errorMessage)
			c.JSON(http.StatusForbidden, response)
			return
		}

		response := helper.APIResponse(helper.MsgFailedToSaveImageToDatabase, http.StatusInternalServerError, "error", errorMessage)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	timestamp := time.Now().Unix()
	filename := file.Filename
	filename = strings.ReplaceAll(filename, " ", "-")
	re := regexp.MustCompile(`[^a-zA-Z0-9._-]`)
	filename = re.ReplaceAllString(filename, "")

	fileName := fmt.Sprintf("%d-%d-%s", userID, timestamp, filename)
	path := fmt.Sprintf("images/campaign-images/%s", fileName)

	src, err := file.Open()
	if err != nil {
		data := gin.H{"is_uploaded": false}
		response := helper.APIResponse(helper.MsgFailedToSaveFileToServer, http.StatusInternalServerError, "error", data)
		c.JSON(http.StatusInternalServerError, response)
		return
	}
	defer src.Close()

	err = storage.UploadFile(c.Request.Context(), src, path, file.Header.Get("Content-Type"))
	if err != nil {
		data := gin.H{"is_uploaded": false}
		response := helper.APIResponse(helper.MsgFailedToSaveFileToServer, http.StatusInternalServerError, "error", data)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	_, err = h.service.SaveCampaignImage(input, path)
	if err != nil {
		errorMessage := gin.H{"errors": err.Error(), "is_uploaded": false}
		response := helper.APIResponse(helper.MsgFailedToSaveImageToDatabase, http.StatusInternalServerError, "error", errorMessage)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	data := gin.H{"is_uploaded": true}
	response := helper.APIResponse(helper.MsgCampaignImageUploadedSuccessfully, http.StatusOK, "success", data)
	c.JSON(http.StatusOK, response)
}
