package transport

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/exepirit/meshtastic-go/pkg/meshtastic/proto"
	"tinygo.org/x/bluetooth"
)

type bleMatcher func(adv bleAdvertisement) bool

var bleAdapterOnce sync.Once
var bleAdapterErr error

func enableBLEAdapter() error {
	bleAdapterOnce.Do(func() {
		bleAdapterErr = bluetooth.DefaultAdapter.Enable()
	})
	return bleAdapterErr
}

func connectBLE(ctx context.Context, mac, name string) (*bleTransport, error) {
	if err := enableBLEAdapter(); err != nil {
		return nil, fmt.Errorf("enable BLE adapter: %w", err)
	}

	match, err := newBLEMatcher(mac, name)
	if err != nil {
		return nil, err
	}

	adv, err := discoverBLEAdvertisement(ctx, match)
	if err != nil {
		return nil, fmt.Errorf("BLE device not found: %w", err)
	}
	if adv.url == "" {
		return nil, fmt.Errorf("BLE device not found")
	}
	slog.Info("ble: connecting", "name", adv.name, "address", adv.addressKey)

	return openBLEDevice(bluetooth.DefaultAdapter, adv.address)
}

func openBLEDevice(adapter *bluetooth.Adapter, address bluetooth.Address) (*bleTransport, error) {
	slog.Debug("ble: connecting", "address", address.String())
	device, err := adapter.Connect(address, bluetooth.ConnectionParams{})
	if err != nil {
		return nil, fmt.Errorf("BLE connect %s: %w", address.String(), err)
	}
	slog.Debug("ble: connected", "address", address.String())
	stabilizeBLEConnection()

	services, err := device.DiscoverServices([]bluetooth.UUID{meshServiceUUID})
	if err != nil {
		_ = device.Disconnect()
		return nil, fmt.Errorf("discover Meshtastic BLE service: %w", err)
	}
	if len(services) == 0 {
		_ = device.Disconnect()
		return nil, fmt.Errorf("no Meshtastic BLE service on %s", address.String())
	}
	slog.Debug("ble: service discovered", "service", meshServiceUUID.String())

	chars, err := services[0].DiscoverCharacteristics([]bluetooth.UUID{
		fromRadioCharUUID, toRadioCharUUID, fromNumCharUUID,
	})
	if err != nil {
		_ = device.Disconnect()
		return nil, fmt.Errorf("discover Meshtastic BLE characteristics: %w", err)
	}
	slog.Debug("ble: characteristics discovered", "count", len(chars))

	fromRadio, toRadio, fromNum, err := mapBLECharacteristics(chars)
	if err != nil {
		_ = device.Disconnect()
		return nil, err
	}

	t := &bleTransport{
		device:    device,
		fromRadio: fromRadio,
		toRadio:   toRadio,
		fromNum:   fromNum,
		packets:   make(chan *proto.FromRadio, blePacketBuffer),
	}
	if err := t.start(); err != nil {
		_ = t.Close()
		return nil, err
	}
	return t, nil
}

func mapBLECharacteristics(chars []bluetooth.DeviceCharacteristic) (fromRadio, toRadio, fromNum bluetooth.DeviceCharacteristic, err error) {
	byUUID := make(map[string]bluetooth.DeviceCharacteristic, len(chars))
	for _, c := range chars {
		byUUID[c.UUID().String()] = c
	}
	var ok bool
	fromRadio, ok = byUUID[fromRadioCharUUID.String()]
	if !ok {
		return fromRadio, toRadio, fromNum, fmt.Errorf("incomplete Meshtastic BLE characteristics")
	}
	toRadio, ok = byUUID[toRadioCharUUID.String()]
	if !ok {
		return fromRadio, toRadio, fromNum, fmt.Errorf("incomplete Meshtastic BLE characteristics")
	}
	fromNum, ok = byUUID[fromNumCharUUID.String()]
	if !ok {
		return fromRadio, toRadio, fromNum, fmt.Errorf("incomplete Meshtastic BLE characteristics")
	}
	return fromRadio, toRadio, fromNum, nil
}

func matchName(name string) bleMatcher {
	return func(adv bleAdvertisement) bool {
		return adv.name == name
	}
}

func matchNoDevice(_ bleAdvertisement) bool {
	return false
}