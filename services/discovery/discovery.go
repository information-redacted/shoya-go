package discovery

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/rueian/rueidis"
	"gitlab.com/george/shoya-go/config"
	"gitlab.com/george/shoya-go/models"
)

var RedisClient rueidis.Client
var RedisCtx = context.Background()

func Main() {
	if config.RuntimeConfig.Discovery == nil {
		log.Fatalf("error reading config: RuntimeConfig.Discovery was nil")
	}

	initializeRedis()

	go instanceCleanup()

	app := fiber.New(fiber.Config{
		ProxyHeader: config.RuntimeConfig.Discovery.Fiber.ProxyHeader,
		Prefork:     false,
	})
	//app.Use(recover.New())
	app.Use(logger.New())
	app.Use(func(c *fiber.Ctx) error {
		k := c.Query("apiKey")
		if k == "" {
			k = c.Get("Authorization")
			if k == "" {
				return c.SendStatus(401)
			}
		}

		if k != config.RuntimeConfig.Discovery.DiscoveryApiKey {
			return c.SendStatus(401)
		}

		return c.Next()
	})

	app.Get("/:instanceId", func(c *fiber.Ctx) error {
		id := c.Params("instanceId")
		i, err := getInstance(id)
		if err != nil {
			if err == NotFoundErr {
				return c.SendStatus(404)
			}

			fmt.Println(err)
			return c.Status(500).JSON(fiber.Map{
				"error":      err.Error(),
				"instanceId": id,
			})
		}

		return c.JSON(i)
	})

	app.Get("/s/:shortName", func(c *fiber.Ctx) error {
		id := c.Params("shortName")
		i, err := findInstanceForShortName(id)
		if err != nil {
			if err == NotFoundErr {
				return c.SendStatus(404)
			}

			fmt.Println(err)
			return c.Status(500).JSON(fiber.Map{
				"error":     err.Error(),
				"shortName": id,
			})
		}

		return c.JSON(i)
	})

	app.Get("/world/:worldId", func(c *fiber.Ctx) error {
		i, err := findInstancesForWorldId(escapeId(c.Params("worldId")), "public", false)
		if err != nil {
			if err == NotFoundErr {
				return c.SendStatus(404)
			}

			fmt.Println(err)
			return c.Status(500).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(i)
	})

	app.Post("/register/:instanceId", func(c *fiber.Ctx) error {
		var capacity int
		id := c.Params("instanceId")
		if _cap := c.Query("capacity"); _cap == "" {
			return c.Status(500).JSON(fiber.Map{
				"error":      "capacity query parameter is required",
				"instanceId": id,
			})
		} else {
			var err error
			capacity, err = strconv.Atoi(_cap)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{
					"error":      err.Error(),
					"instanceId": id,
				})
			}
		}

		l, err := models.ParseLocationString(id)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"error":      err.Error(),
				"instanceId": id,
			})
		}

		i, err := registerInstance(l.ID, l.LocationString, l.WorldID, l.InstanceType, l.OwnerID, capacity)
		if err != nil {
			fmt.Println(err)
			return c.Status(500).JSON(fiber.Map{
				"error":      err.Error(),
				"instanceId": id,
			})
		}

		return c.JSON(i)
	})

	app.Post("/ping/:instanceId", func(c *fiber.Ctx) error {
		i := c.Params("instanceId")
		err := pingInstance(i)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"error":      err.Error(),
				"instanceId": i,
			})
		}

		return c.SendStatus(200)
	})

	app.Post("/unregister/:instanceId", func(c *fiber.Ctx) error {
		i := c.Params("instanceId")
		err := unregisterInstance(i)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"error":      err.Error(),
				"instanceId": i,
			})
		}

		return c.SendStatus(200)
	})

	app.Get("/player/:playerId", func(c *fiber.Ctx) error {
		p := c.Params("playerId")
		i, err := findInstancesPlayerIsIn(p)
		if err != nil {
			if err == NotFoundErr {
				return c.SendStatus(404)
			}

			fmt.Println(err)
			return c.Status(500).JSON(fiber.Map{
				"error":    err.Error(),
				"playerId": p,
			})
		}
		return c.JSON(i)
	})

	app.Put("/player/:instanceId/:playerId", func(c *fiber.Ctx) error {
		i := c.Params("instanceId")
		p := c.Params("playerId")

		err := addPlayer(i, p)

		if err != nil {
			fmt.Println(err)
			return c.Status(500).JSON(fiber.Map{
				"error":      err.Error(),
				"instanceId": i,
			})
		}

		return c.SendStatus(200)
	})

	app.Delete("/player/:instanceId/:playerId", func(c *fiber.Ctx) error {
		i := c.Params("instanceId")
		p := c.Params("playerId")

		err := removePlayer(i, p)

		if err != nil {
			fmt.Println(err)
			return c.Status(500).JSON(fiber.Map{
				"error":      err.Error(),
				"instanceId": i,
			})
		}

		return c.SendStatus(200)
	})

	log.Fatal(app.Listen(config.RuntimeConfig.Discovery.Fiber.ListenAddress))
}

func initializeRedis() {
	redisClient, err := rueidis.NewClient(rueidis.ClientOption{
		Username:    "default",
		Password:    config.RuntimeConfig.Discovery.Redis.Password,
		InitAddress: []string{config.RuntimeConfig.Discovery.Redis.Host},
	})

	if err != nil {
		panic(err)
	}

	RedisClient = redisClient

	if err = RedisClient.Do(context.Background(), redisClient.B().FtInfo().Index("instanceShortNameIdx").Build()).Error(); err != nil {
		log.Println("Creating index instanceShortNameIdx")
		RedisClient.Do(context.Background(), RedisClient.B().FtCreate().
			Index("instanceShortNameIdx").OnJson().Schema().
			FieldName("$.shortName").As("shortName").Tag().
			FieldName("$.secureName").As("secureName").Tag().Build())
	}

	if err = RedisClient.Do(context.Background(), RedisClient.B().FtInfo().Index("instanceWorldIdIdx").Build()).Error(); err != nil {
		log.Println("Creating index instanceWorldIdIdx")
		RedisClient.Do(context.Background(), RedisClient.B().FtCreate().
			Index("instanceWorldIdIdx").OnJson().Schema().
			FieldName("$.worldId").As("worldId").Tag().
			FieldName("$.instanceType").As("instanceType").Tag().
			FieldName("$.overCapacity").As("overCapacity").Tag().
			Build())
	}

	if err = RedisClient.Do(context.Background(), RedisClient.B().FtInfo().Index("instancePlayersIdx").Build()).Error(); err != nil {
		log.Println("Creating index instancePlayersIdx")
		RedisClient.Do(context.Background(), RedisClient.B().FtCreate().
			Index("instancePlayersIdx").OnJson().Schema().
			FieldName("$.players[0:]").As("players").Tag().
			Build())
	}

	if err = RedisClient.Do(context.Background(), RedisClient.B().FtInfo().Index("instancePingTimeIdx").Build()).Error(); err != nil {
		log.Println("Creating index instancePingTimeIdx")
		RedisClient.Do(context.Background(), RedisClient.B().FtCreate().
			Index("instancePingTimeIdx").OnJson().Schema().
			FieldName("$.lastPing").As("lastPing").Numeric().
			Build())
	}
}

func instanceCleanup() {
	var currentTime = int64(0)
	for {
		currentTime = time.Now().UTC().Unix()

		arr, err := RedisClient.Do(RedisCtx, RedisClient.B().FtSearch().Index("instancePingTimeIdx").Query(fmt.Sprintf("@lastPing:[-inf %d]", currentTime-3600)).Build()).ToArray()
		if err != nil {
			log.Println(err)
			time.Sleep(30 * time.Second)
			continue
		}

		var n int64
		var p []FtSearchResult
		n, p, err = parseFtSearch(arr)

		if n >= 1 {
			log.Printf("Cleanup Routine - Cleaned up %d instances.", n)
		}

		for _, val := range p {
			err = RedisClient.Do(RedisCtx, RedisClient.B().Del().Key(val.Key).Build()).Error()
			if err != nil {
				log.Printf("error deleting instance: %s\n", err.Error())
			}
		}

		time.Sleep(30 * time.Second)
	}
}
