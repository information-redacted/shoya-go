package main

import (
	"github.com/gofiber/fiber/v2"
	"gitlab.com/george/shoya-go/config"
	"gitlab.com/george/shoya-go/models"
	"gorm.io/gorm/clause"
	"strconv"
	"time"
)

func photonRoutes(router *fiber.App) {
	photon := router.Group("/photon")
	photon.Get("/ns", photonSecret, doNsAuth)
	photon.Get("/validateJoin", photonSecret, doJoinTokenValidation)
	photon.Get("/user", photonSecret, doPropertyUpdate)
	photon.Get("/getConfig", photonSecret, getPhotonConfig)
	photon.Get("/playerLeft", photonSecret, doLeaveCallback)
	photon.Get("/gameClosed", photonSecret, doGameClose)
}

var PhotonInvalidParametersResponse = fiber.Map{"ResultCode": 3}
var PhotonCustomAuthFailedResponse = fiber.Map{"ResultCode": 2}
var PhotonCustomAuthSuccessResponse = fiber.Map{"ResultCode": 1}

func photonSecret(c *fiber.Ctx) error {
	if c.Query("secret") != config.ApiConfiguration.PhotonSecret.Get() {
		return c.JSON(fiber.Map{"ResultCode": 3})
	}
	return c.Next()
}

func doNsAuth(c *fiber.Ctx) error {
	t := c.Query("token")
	u := c.Query("user")
	if t == "" || u == "" {
		return c.JSON(PhotonInvalidParametersResponse)
	}

	uid, err := models.ValidateAuthCookie(t, c.IP(), false, true)
	if err != nil || uid != u {
		return c.JSON(PhotonCustomAuthFailedResponse)
	}

	return c.JSON(PhotonCustomAuthSuccessResponse)
}

func doJoinTokenValidation(c *fiber.Ctx) error {
	t := c.Query("jwt")
	l := c.Query("roomId")
	oc := boolConvert(c.Query("onCreate"))

	if t == "" || l == "" {
		return c.JSON(models.PhotonValidateJoinJWTResponse{Valid: false})
	}

	claims, err := models.ValidateJoinToken(t)
	if err != nil {
		return c.JSON(models.PhotonValidateJoinJWTResponse{Valid: false})
	}

	if claims.Location != l {
		return c.JSON(models.PhotonValidateJoinJWTResponse{Valid: false})
	}

	var u models.User
	tx := config.DB.Model(&models.User{}).Preload(clause.Associations).
		Preload("CurrentAvatar.Image").
		Preload("FallbackAvatar.Image").
		Preload("CurrentAvatar.UnityPackages.File").
		Preload("FallbackAvatar.UnityPackages.File").
		Where("id = ?", claims.UserId).First(&u)
	if tx.Error != nil {
		return c.JSON(models.PhotonValidateJoinJWTResponse{Valid: false})
	}

	r := models.PhotonValidateJoinJWTResponse{
		Time:  strconv.Itoa(int(time.Now().Unix())),
		Valid: true,
		IP:    claims.IP,
	}
	err = r.FillFromUser(&u)
	if err != nil {
		return c.Status(500).JSON(models.MakeErrorResponse(err.Error(), 500))
	}

	if oc {
		r.WorldAuthor = claims.WorldAuthorId
		r.WorldCapacity = claims.WorldCapacity
		r.InstanceCreator = claims.InstanceOwnerId
	}

	if config.ApiConfiguration.DiscoveryServiceEnabled.Get() {
		DiscoveryService.AddPlayerToInstance(u.ID, l)
	}
	return c.JSON(r)
}

func doLeaveCallback(c *fiber.Ctx) error {
	l := c.Query("roomId")
	u := c.Query("userId")

	if config.ApiConfiguration.DiscoveryServiceEnabled.Get() {
		DiscoveryService.RemovePlayerFromInstance(u, l)
	}
	return c.SendStatus(200)
}

func doGameClose(c *fiber.Ctx) error {
	l := c.Query("roomId")

	if config.ApiConfiguration.DiscoveryServiceEnabled.Get() {
		DiscoveryService.UnregisterInstance(l)
	}
	return c.SendStatus(200)
}

func doPropertyUpdate(c *fiber.Ctx) error {
	var uid = c.Query("userId")
	var u models.User
	tx := config.DB.Model(&models.User{}).Preload(clause.Associations).
		Preload("CurrentAvatar.Image").
		Preload("FallbackAvatar.Image").
		Preload("CurrentAvatar.UnityPackages.File").
		Preload("FallbackAvatar.UnityPackages.File").
		Where("id = ?", uid).First(&u)
	if tx.Error != nil {
		return c.JSON(models.PhotonValidateJoinJWTResponse{Valid: false})
	}

	r := models.PhotonValidateJoinJWTResponse{
		Time:  strconv.Itoa(int(time.Now().Unix())),
		Valid: true,
		IP:    "notset",
	}
	err := r.FillFromUser(&u)
	if err != nil {
		return c.Status(500).JSON(models.MakeErrorResponse(err.Error(), 500))
	}
	return c.JSON(r)
}

func getPhotonConfig(c *fiber.Ctx) error {
	return c.JSON(&models.PhotonConfig{
		MaxAccountsPerIPAddress: int(config.ApiConfiguration.PhotonSettingMaxAccountsPerIpAddress.Get()),
		RateLimitList: map[int]int{
			// This list of rate-limits is hard-coded for now; The following are real-world values as seen
			// in official servers.
			//
			// The object consists of an event code & how many times it can be raised per second.
			1:   60,  // Voice Data
			3:   5,   // Request for past event synchronization (as part of world join)
			4:   200, // Response for past event synchronization
			5:   50,  // "FIN" packet for past event synchronization
			6:   400, // VrcEvent (a.k.a, RPCs)
			7:   500, // Unreliable sync (e.g: movement)
			8:   1,   // Interest Management
			9:   75,  // Reliable sync (e.g.: Udon variables)
			33:  2,   // Moderation
			40:  1,   // Update partial actor properties
			42:  1,   // Update partial actor properties (currently only used for height [24-03-22])
			202: 1,   // Instantiation
			209: 20,  // Request for ownership transfer
			210: 90,  // Ownership transfer
		},
		RatelimiterActive: false,
	})
}