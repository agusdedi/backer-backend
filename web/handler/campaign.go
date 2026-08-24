package handler

import (
	"backer/campaign"
	"backer/storage"
	"backer/user"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	campaignRedirectPath  = "/campaigns"
	campaignEditTemplate  = "campaign_edit.html"
	campaignIndexTemplate = "campaign_index.html"
	campaignImageTemplate = "campaign_image.html"
	campaignNewTemplate   = "campaign_new.html"
	campaignShowTemplate  = "campaign_show.html"
)

type campaignHandler struct {
	campaignService campaign.Service
	userService     user.Service
}

func NewCampaignHandler(campaignService campaign.Service, userService user.Service) *campaignHandler {
	return &campaignHandler{campaignService, userService}
}

func (h *campaignHandler) Index(c *gin.Context) {
	campaigns, err := h.campaignService.GetCampaigns(0)
	if err != nil {
		c.HTML(http.StatusInternalServerError, errorTemplate, nil)
		return
	}

	c.HTML(http.StatusOK, campaignIndexTemplate, gin.H{"campaigns": campaigns})
}

func (h *campaignHandler) New(c *gin.Context) {
	users, err := h.userService.GetAllUsers()
	if err != nil {
		c.HTML(http.StatusInternalServerError, errorTemplate, nil)
		return
	}

	input := campaign.FormCreateCampaignInput{}
	input.Users = users

	c.HTML(http.StatusOK, campaignNewTemplate, input)
}

func (h *campaignHandler) Create(c *gin.Context) {
	var input campaign.FormCreateCampaignInput

	err := c.ShouldBind(&input)
	if err != nil {
		users, e := h.userService.GetAllUsers()
		if e != nil {
			c.HTML(http.StatusInternalServerError, errorTemplate, nil)
			return
		}

		input.Users = users
		input.Error = err

		c.HTML(http.StatusOK, campaignNewTemplate, input)
		return
	}

	user, err := h.userService.GetUserByID(input.UserID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, errorTemplate, nil)
		return
	}

	createCampaignInput := campaign.CreateCampaignInput{}
	createCampaignInput.Name = input.Name
	createCampaignInput.ShortDescription = input.ShortDescription
	createCampaignInput.Description = input.Description
	createCampaignInput.GoalAmount = input.GoalAmount
	createCampaignInput.Perks = input.Perks
	createCampaignInput.User = user

	_, err = h.campaignService.CreateCampaign(createCampaignInput)
	if err != nil {
		c.HTML(http.StatusInternalServerError, errorTemplate, nil)
		return
	}

	c.Redirect(http.StatusFound, campaignRedirectPath)
}

func (h *campaignHandler) NewImage(c *gin.Context) {
	idParam := c.Param("id")
	id, _ := strconv.Atoi(idParam)

	c.HTML(http.StatusOK, campaignImageTemplate, gin.H{"ID": id})
}

func (h *campaignHandler) CreateImage(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.HTML(http.StatusInternalServerError, errorTemplate, nil)
		return
	}

	idParam := c.Param("id")
	id, _ := strconv.Atoi(idParam)

	existingCampaign, err := h.campaignService.GetCampaignByID(campaign.GetCampaignDetailInput{ID: id})
	if err != nil {
		c.HTML(http.StatusInternalServerError, errorTemplate, nil)
		return
	}

	userID := existingCampaign.UserID

	timestamp := time.Now().Unix()
	filename := file.Filename
	filename = strings.ReplaceAll(filename, " ", "-")
	re := regexp.MustCompile(`[^a-zA-Z0-9._-]`)
	filename = re.ReplaceAllString(filename, "")

	fileName := fmt.Sprintf("%d-%d-%s", userID, timestamp, filename)
	path := fmt.Sprintf("images/campaign-images/%s", fileName)

	src, err := file.Open()
	if err != nil {
		c.HTML(http.StatusInternalServerError, errorTemplate, nil)
		return
	}
	defer src.Close()

	err = storage.UploadFile(c.Request.Context(), src, path, file.Header.Get("Content-Type"))
	if err != nil {
		c.HTML(http.StatusInternalServerError, errorTemplate, nil)
		return
	}

	createCampaignImageInput := campaign.CreateCampaignImageInput{}
	createCampaignImageInput.CampaignID = id
	createCampaignImageInput.IsPrimary = true

	userCampaign, err := h.userService.GetUserByID(userID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, errorTemplate, nil)
		return
	}

	createCampaignImageInput.User = userCampaign

	_, err = h.campaignService.SaveCampaignImage(createCampaignImageInput, path)
	if err != nil {
		c.HTML(http.StatusInternalServerError, errorTemplate, nil)
		return
	}

	c.Redirect(http.StatusFound, campaignRedirectPath)
}

func (h *campaignHandler) Edit(c *gin.Context) {
	idParam := c.Param("id")
	id, _ := strconv.Atoi(idParam)

	existingCampaign, err := h.campaignService.GetCampaignByID(campaign.GetCampaignDetailInput{ID: id})
	if err != nil {
		c.HTML(http.StatusInternalServerError, errorTemplate, nil)
		return
	}

	input := campaign.FormUpdateCampaignInput{}
	input.ID = existingCampaign.ID
	input.Name = existingCampaign.Name
	input.ShortDescription = existingCampaign.ShortDescription
	input.Description = existingCampaign.Description
	input.GoalAmount = existingCampaign.GoalAmount
	input.Perks = existingCampaign.Perks

	c.HTML(http.StatusOK, campaignEditTemplate, input)
}

func (h *campaignHandler) Update(c *gin.Context) {
	idParam := c.Param("id")
	id, _ := strconv.Atoi(idParam)

	var input campaign.FormUpdateCampaignInput

	err := c.ShouldBind(&input)
	if err != nil {
		input.ID = id
		input.Error = err

		c.HTML(http.StatusInternalServerError, errorTemplate, input)
		return
	}

	existingCampaign, err := h.campaignService.GetCampaignByID(campaign.GetCampaignDetailInput{ID: id})
	if err != nil {
		c.HTML(http.StatusInternalServerError, errorTemplate, nil)
		return
	}

	userID := existingCampaign.UserID

	userCampaign, err := h.userService.GetUserByID(userID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, errorTemplate, nil)
		return
	}

	updateCampaignInput := campaign.CreateCampaignInput{}
	updateCampaignInput.Name = input.Name
	updateCampaignInput.ShortDescription = input.ShortDescription
	updateCampaignInput.Description = input.Description
	updateCampaignInput.GoalAmount = input.GoalAmount
	updateCampaignInput.Perks = input.Perks
	updateCampaignInput.User = userCampaign

	_, err = h.campaignService.UpdateCampaign(campaign.GetCampaignDetailInput{ID: id}, updateCampaignInput)
	if err != nil {
		c.HTML(http.StatusInternalServerError, errorTemplate, nil)
		return
	}

	c.Redirect(http.StatusFound, campaignRedirectPath)
}

func (h *campaignHandler) Show(c *gin.Context) {
	idParam := c.Param("id")
	id, _ := strconv.Atoi(idParam)

	existingCampaign, err := h.campaignService.GetCampaignByID(campaign.GetCampaignDetailInput{ID: id})
	if err != nil {
		c.HTML(http.StatusInternalServerError, errorTemplate, nil)
		return
	}

	c.HTML(http.StatusOK, campaignShowTemplate, existingCampaign)
}
