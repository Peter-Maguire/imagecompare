package main

import (
	"fmt"
	"github.com/eclipse/paho.mqtt.golang"
	"image"
	"image/jpeg"
	"net/http"
	"os"
	"time"
)

var deviceConfig = `{
  "device": {
    "identifiers": [
      "washingmachine"
    ],
	"model": "H7 W945WB",
	"manufacturer": "Hotpoint",
    "name": "Washing Machine"
  },
  "device_class": "running",
  "name": "Running",
  "object_id": "washing_machine_running",
  "origin": {
    "name": "Big P Image Compare",
    "sw": "0.0.0",
    "url": "https://bi.gp"
  },
  "state_topic": "imagecompare/washingmachine/state",
  "payload_off": "off",
  "payload_on": "on",
  "availability_topic": "imagecompare/washingmachine/availability",
  "payload_available": "online",
  "payload_not_available": "offline",
  "enabled_by_default": true,
  "entity_category": "diagnostic",

  "unique_id": "imagecompare_washingmachine"
}`

var dayTimeThreshold = float64(21000)
var nightTimeThreshold = float64(17000)

var imageUrl = os.Getenv("IMAGE_URL")

var mqttBroker = os.Getenv("MQTT_HOST")
var mqttUsername = os.Getenv("MQTT_USERNAME")
var mqttPassword = os.Getenv("MQTT_PASSWORD")

type SubImage interface {
	SubImage(r image.Rectangle) image.Image
}

func main() {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(mqttBroker)
	opts.SetClientID("washingmachine-checker")
	opts.SetUsername(mqttUsername)
	opts.SetPassword(mqttPassword)
	client := mqtt.NewClient(opts)
	token := client.Connect()
	err := token.Error()
	fmt.Println(err)
	token.Wait()
	fmt.Println("Starting loop")
	go runLoop(client)
	forever := make(chan bool)
	<-forever
}

func runLoop(client mqtt.Client) {
	ticker := time.NewTicker(1 * time.Minute)
	times := 0
	lastState := ""
	for {
		state := isWashingOn()
		client.Publish("homeassistant/binary_sensor/washingmachine/running/config", 0, false, deviceConfig)

		if lastState == state {
			times++
		} else {
			lastState = state
			times = 1
		}

		if times > 3 {
			if state == "offline" {
				client.Publish("imagecompare/washingmachine/availability", 0, false, "offline")
			} else {
				client.Publish("imagecompare/washingmachine/state", 0, false, state)
				client.Publish("imagecompare/washingmachine/availability", 0, false, "online")
			}
		}
		<-ticker.C
	}
}

func isWashingOn() string {
	res, err := http.DefaultClient.Get(imageUrl)
	if err != nil {
		fmt.Println(err)
		return "offline"
	}
	img, err := jpeg.Decode(res.Body)
	res.Body.Close()
	if err != nil {
		fmt.Println(err)
		return "offline"
	}

	averageDisplay := getPixelAveragePixelColour(getAreaOfInterest(img))
	averageControl := getRedGreenDifference(getComparisonArea(img))
	daytime := averageControl > 100
	fmt.Println("Daytime: ", daytime)

	washingOn := (daytime && averageDisplay > dayTimeThreshold) || (!daytime && averageDisplay > nightTimeThreshold)
	fmt.Println(averageDisplay)
	if washingOn {
		return "on"
	}
	return "off"
}

func getAreaOfInterest(img image.Image) image.Image {
	pngImage := img.(SubImage)
	subImage := pngImage.SubImage(image.Rect(162, 273, 162+27, 273+20))
	return subImage
}

func getComparisonArea(img image.Image) image.Image {
	pngImage := img.(SubImage)
	subImage := pngImage.SubImage(image.Rect(394, 19, 394+27, 19+20))
	return subImage
}

func getPixelAveragePixelColour(img image.Image) float64 {
	bounds := img.Bounds()

	total := uint64(0)
	count := 0
	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			colourHere := img.At(x, y)
			r, g, b, _ := colourHere.RGBA()
			total += uint64(r) + uint64(g) + uint64(b)
			count++
		}
	}
	return float64(total) / float64(count*3)
}

func getRedGreenDifference(img image.Image) float64 {
	bounds := img.Bounds()

	redTotal := uint64(0)
	greenTotal := uint64(0)
	count := 0
	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			colourHere := img.At(x, y)
			r, g, _, _ := colourHere.RGBA()
			redTotal += uint64(r)
			greenTotal += uint64(g)
			count++
		}
	}
	redAverage := float64(redTotal) / float64(count)
	greenAverage := float64(greenTotal) / float64(count)
	//fmt.Println(total, count)
	return redAverage - greenAverage
}
