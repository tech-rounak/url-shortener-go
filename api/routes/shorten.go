package routes

import (
	"os"
	"strconv"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	database "github.com/tech-rounak/url-shortener/database"
	helpers "github.com/tech-rounak/url-shortener/helpers"
)

type request struct {
	URL         string        `json:"url"`
	CustomShort string        `json:"customShort"`
	Expiry      time.Duration `json:"expiry"`
}

type response struct {
	URL            string        `json:"url"`
	CustomShort    string        `json:"customShort"`
	Expiry         time.Duration `json:"expiry"`
	XRateRemaining int           `json:"xRateRemaining"`
	XRateLimitRest time.Duration `json:"xRateLimitRest"`
}

func ShortenURL(c *fiber.Ctx) error {
	body := new(request)

	if err := c.BodyParser(body); err != nil {
		return c.Status(fiber.ErrBadRequest.Code).JSON(fiber.Map{"error": err.Error()})
	}

	// implement rate limit
	rdb := database.CreateClient(1)
	defer rdb.Close()
	val, err := rdb.Get(database.Ctx, c.IP()).Result()

	if err != nil {
		_ = rdb.Set(database.Ctx, c.IP(), os.Getenv("API_QUOTA"), 30*60*time.Second).Err()
	} else {
		val, _ = rdb.Get(database.Ctx, c.IP()).Result()
		valInt, _ := strconv.Atoi(val)

		if valInt <= 0 {
			limit, _ := rdb.TTL(database.Ctx, c.IP()).Result()
			return c.Status(fiber.ErrServiceUnavailable.Code).JSON(fiber.Map{
				"error":           "Rate limit exceeded",
				"rate_limit_rest": limit / time.Nanosecond / time.Minute,
			})
		}
	}
	// check if input is an actual URL
	if !govalidator.IsURL(body.URL) {
		return c.Status(fiber.ErrBadRequest.Code).JSON(fiber.Map{"error": "Invalid url"})
	}
	// check for domain error
	if !helpers.RemoveDomainError(body.URL) {
		return c.Status(fiber.ErrBadRequest.Code).JSON(fiber.Map{"error": "Invalid domain"})
	}
	// enforce http ssl

	body.URL = helpers.EnforceHTTP(body.URL)

	var id string
	if body.CustomShort == "" {
		id = uuid.New().String()[:6]
	} else {
		id = body.CustomShort
	}

	r := database.CreateClient(0)
	defer r.Close()
	val, _ = r.Get(database.Ctx, id).Result()
	if val != "" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "URL custom is in use"})
	}
	if body.Expiry == 0 {
		body.Expiry = 24
	}
	err = r.Set(database.Ctx, id, body.URL, body.Expiry*3600*time.Second).Err()

	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Server error"})
	}
	resp := response{
		URL:            body.URL,
		CustomShort:    "",
		Expiry:         body.Expiry,
		XRateRemaining: 10,
		XRateLimitRest: 10,
	}
	rdb.Decr(database.Ctx, c.IP())

	val, _ = rdb.Get(database.Ctx, c.IP()).Result()

	resp.XRateRemaining, _ = strconv.Atoi(val)

	ttl, _ := rdb.TTL(database.Ctx, c.IP()).Result()
	resp.XRateLimitRest = ttl / time.Nanosecond / time.Minute
	resp.CustomShort = os.Getenv("DOMAIN") + "/" + id

	return c.Status(fiber.StatusOK).JSON(resp)
}
