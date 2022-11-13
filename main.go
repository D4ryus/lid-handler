package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/godbus/dbus/v5"
)

var brightnessFile = "/sys/class/backlight/intel_backlight/brightness"

func getBrightness() (float64, error) {
	data, err := ioutil.ReadFile(brightnessFile)
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
}

func setBrightness(value float64) error {
	return ioutil.WriteFile(brightnessFile, strconv.AppendFloat([]byte(nil), value, 'f', 0, 64), 0)
}

func onSignal(obj dbus.BusObject, signal *dbus.Signal, onLidChanged func(bool) error) error {
	if signal.Name != "org.freedesktop.DBus.Properties.PropertiesChanged" {
		return fmt.Errorf("unexpected signal: %v", signal.Name)
	}
	var name string
	var changed map[string]dbus.Variant
	var invalidated []string
	if err := dbus.Store(signal.Body, &name, &changed, &invalidated); err != nil {
		return fmt.Errorf("invalid PropertiesChanged body")
	}
	if variant, ok := changed["LidIsClosed"]; ok {
		if closed, ok := variant.Value().(bool); ok {
			return onLidChanged(closed)
		}
		return fmt.Errorf("unexpected LidIsClosed type")
	}
	for _, invalid := range invalidated {
		if invalid != "LidIsClosed" {
			continue
		}
		var closed bool
		if err := obj.StoreProperty("org.freedesktop.UPower.LidIsClosed", &closed); err != nil {
			return fmt.Errorf("could not store LidIsClosed property: %v", err)
		}
		return onLidChanged(closed)
	}
	return fmt.Errorf("LidIsClosed was not invalidated")
}

func usage(code int) {
	log.Printf("usage: %s [brightness-file]\n", os.Args[0])
	log.Printf("  If brightness-file is not specified it defaults to %s", brightnessFile)
	os.Exit(code)
}

func main() {
	log.SetFlags(0) // Systemd will prefix with timestamps

	switch len(os.Args) {
	case 1:
		break
	case 2:
		switch os.Args[1] {
		case "-h", "--help":
			usage(0)
		default:
			brightnessFile = os.Args[1]
		}
	default:
		log.Printf("Error: Invalid arguments specified!\n")
		usage(1)
	}

	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		log.Fatal("Failed to connect to system bus:", err)
	}
	defer conn.Close()

	obj := conn.Object("org.freedesktop.UPower", "/org/freedesktop/UPower")

	var present bool
	if err := obj.StoreProperty("org.freedesktop.UPower.LidIsPresent", &present); err != nil {
		log.Fatal(err)
	}
	if !present {
		log.Fatal("No Lid present")
	}

	brightness, err := getBrightness()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Brightness on startup:", brightness)
	onLidChanged := func(closed bool) (err error) {
		if closed {
			brightness, err = getBrightness()
			if err != nil {
				log.Fatal(err)
			}
			log.Println("Lid closed, saved brightness:", brightness)
			if err := setBrightness(0); err != nil {
				log.Fatal(err)
			}
		} else {
			log.Println("Lid opened, restoring brightness:", brightness)
			if err = setBrightness(brightness); err != nil {
				log.Fatal(err)
			}
		}
		return
	}

	if err := conn.AddMatchSignal(
		dbus.WithMatchObjectPath("/org/freedesktop/UPower"),
		dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		dbus.WithMatchSender("org.freedesktop.UPower"),
		dbus.WithMatchMember("PropertiesChanged"),
		dbus.WithMatchArg(0, "org.freedesktop.UPower"),
	); err != nil {
		log.Fatal(err)
	}
	c := make(chan *dbus.Signal, 10)
	conn.Signal(c)
	for signal := range c {
		if err := onSignal(obj, signal, onLidChanged); err != nil {
			log.Println(err)
		}
	}
}
