package api

import (
	"github.com/gofiber/fiber/v2"
	"gitlab.com/george/shoya-go/config"
	"gitlab.com/george/shoya-go/models"
	"gitlab.com/george/shoya-go/models/service_types"
	"gitlab.com/george/shoya-go/services/presence/presence_client"
	"os"
	"strings"
	"time"
)

func systemRoutes(router *fiber.App) {
	router.Get("/ping", getPing)
	router.Get("/health", getHealth)
	router.Get("/pigascii", getPig)
	router.Get("/time", getTime)
	router.Get("/config", getConfig)

	router.Get("/infoPush", ApiKeyMiddleware, AuthMiddleware, getInfoPush)
	router.Put("/logout", ApiKeyMiddleware, AuthMiddleware, putLogout)

	router.Get("/visits", getVisits)
	router.Put("/visits", ApiKeyMiddleware, AuthMiddleware, putVisits)

	router.Put("/joins", ApiKeyMiddleware, AuthMiddleware, putJoins)

	router.Get("/m_autoConfig", getAutoConfig)
}

func getHealth(c *fiber.Ctx) error {
	var s = "ok"

	if !healthStatus.Postgres.Ok || !healthStatus.Redis.Ok || !healthStatus.Config.Ok {
		s = "error"
	}

	return c.Status(200).JSON(fiber.Map{
		"status":     s,
		"serverName": config.ApiConfiguration.ServerName.Get(), // Only here for compatibility.
		"details":    healthStatus,
		"serverInfo": fiber.Map{
			"serverName": config.ApiConfiguration.ServerName.Get(),
			"core":       os.Getppid(),
			"fork":       os.Getpid(),
		},
	})
}

func getConfig(c *fiber.Ctx) error {
	// Add cookie to response
	c.Cookie(&fiber.Cookie{
		Name:     "apiKey",
		Value:    config.ApiConfiguration.ApiKey.Get(),
		SameSite: "disabled",
	})
	return c.JSON(config.NewApiConfigResponse(&config.ApiConfiguration))
}

func getPing(c *fiber.Ctx) error {
	return c.JSON("pong")
}

func putLogout(c *fiber.Ctx) error {
	c.Cookie(&fiber.Cookie{
		Name:     "auth",
		Value:    "",
		Expires:  time.Now().Add(time.Hour * -1),
		SameSite: "disabled",
	})
	c.Cookie(&fiber.Cookie{
		Name:     "twoFactorAuth",
		Value:    "",
		Expires:  time.Now().Add(time.Hour * -1),
		SameSite: "disabled",
	})
	return c.Status(200).JSON(fiber.Map{
		"status": "ok",
	})
}

func getTime(c *fiber.Ctx) error {
	return c.JSON(time.Now().UTC().Format(time.RFC3339))
}

func getInfoPush(c *fiber.Ctx) error {
	// TODO: Refactor to fix a whole host of issues (e.g. duplicate entries)
	//goland:noinspection GoPreferNilSlice
	toPush := []config.ApiInfoPush{}
	requiredTags := strings.Split(c.Query("require"), ",")
	includedTags := strings.Split(c.Query("include"), ",")

	for _, push := range config.ApiConfiguration.InfoPushes.Get() {
		for _, pushed := range toPush {
			if push.Id == pushed.Id {
				break
			}
		}

	nextPush:
		for _, tag := range requiredTags {
			for _, pushTag := range push.Tags {
				if tag == pushTag {
					toPush = append(toPush, push)
					break nextPush
				}
			}
		}

	nextPush2:
		for _, tag := range includedTags {
			for _, pushTag := range push.Tags {
				if tag == pushTag {
					toPush = append(toPush, push)
					break nextPush2
				}
			}
		}
	}

	return c.JSON(toPush)
}

// TODO: Implement active user count
func getVisits(c *fiber.Ctx) error {
	return c.JSON(0)
}

func putVisits(c *fiber.Ctx) error {
	var u = c.Locals("user").(*models.User)
	var r PutVisitsRequest
	var err error

	if err = c.BodyParser(&r); err != nil {
		return c.Status(500).JSON(models.MakeErrorResponse(err.Error(), 500))
	}

	if (r.UserId != "" && r.UserId != u.ID) && !u.IsStaff() {
		return c.Status(400).JSON(models.MakeErrorResponse("can't change someone else's presence", 400))
	}

	if config.ApiConfiguration.DiscoveryServiceEnabled.Get() {
		i := DiscoveryService.GetInstance(r.WorldId)
		if i == nil {
			return c.Status(400).JSON(models.MakeErrorResponse("can't change presence to an instance that doesn't exist", 400))
		}

		DiscoveryService.PingInstance(r.WorldId)
	}

	if config.ApiConfiguration.PresenceServiceEnabled.Get() {
		err = presence_client.PresenceService.UpdateStateForUser(u.ID, service_types.UserStateOnline)
		if err != nil {
			return c.Status(500).JSON(models.MakeErrorResponse(err.Error(), 500))
		}
		err = presence_client.PresenceService.UpdateLastSeenForUser(u.ID, time.Now())
		if err != nil {
			return c.Status(500).JSON(models.MakeErrorResponse(err.Error(), 500))
		}
	}

	return c.JSON(fiber.Map{
		"success": fiber.Map{
			"message":     "User pinged room",
			"status_code": 200,
		},
	})
}

func putJoins(c *fiber.Ctx) error {
	var u = c.Locals("user").(*models.User)
	var r PutJoinsRequest
	var err error

	if err = c.BodyParser(&r); err != nil {
		return c.Status(500).JSON(models.MakeErrorResponse(err.Error(), 500))
	}

	if (r.UserId != "" && r.UserId != u.ID) && !u.IsStaff() {
		return c.Status(400).JSON(models.MakeErrorResponse("can't change someone else's presence", 400))
	}

	if config.ApiConfiguration.PresenceServiceEnabled.Get() {
		err = presence_client.PresenceService.UpdateStateForUser(u.ID, service_types.UserStateOnline)
		if err != nil {
			return c.Status(500).JSON(models.MakeErrorResponse(err.Error(), 500))
		}
		err = presence_client.PresenceService.UpdateInstanceForUser(u.ID, r.WorldId)
		if err != nil {
			return c.Status(500).JSON(models.MakeErrorResponse(err.Error(), 500))
		}

		err = presence_client.PresenceService.UpdateLastSeenForUser(u.ID, time.Now())
		if err != nil {
			return c.Status(500).JSON(models.MakeErrorResponse(err.Error(), 500))
		}
	}

	return c.JSON(fiber.Map{
		"success": fiber.Map{
			"message":     "User joined room",
			"status_code": 200,
		},
	})
}

func getAutoConfig(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"apiUrl":         config.ApiConfiguration.AutoConfigApiUrl.Get(),
		"websocketUrl":   config.ApiConfiguration.AutoConfigWebsocketUrl.Get(),
		"nameServerHost": config.ApiConfiguration.AutoConfigNameServerHost.Get(),
	})
}

func getPig(c *fiber.Ctx) error {
	return c.SendString("{\"pig\":\"\\n             __,---.__\\n        __,-'         `-.\\n       /_ /_,'           \\\\&\\n       _,''               \\\\\\n      (\\\")            .    |\\n pig   ``--|__|--..-'`.__|\\n\"}\n")
}
