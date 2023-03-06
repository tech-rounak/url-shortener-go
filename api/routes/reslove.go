package routes

import (
	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	database "github.com/tech-rounak/url-shortener/database"
)

func ResolveURL(c *fiber.Ctx) error {
	url := c.Params("url")
	rdb := database.CreateClient(0)
	defer rdb.Close()

	value, err := rdb.Get(database.Ctx, url).Result()
	if err == redis.Nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Short Not found in database"})
	} else if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "db connection erro"})
	}
	rInr := database.CreateClient(1)
	defer rInr.Close()

	_ = rInr.Incr(database.Ctx, "counter")
	return c.Redirect(value, 301)

}
