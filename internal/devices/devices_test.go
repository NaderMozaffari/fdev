package devices

import "testing"

func TestParseADB(t *testing.T) {
	out := `* daemon not running; starting now at tcp:5037
* daemon started successfully
List of devices attached
adb-5d9c2e71-k8wqzt._adb-tls-connect._tcp device product:dm3qxxx model:SM-S918B device:dm3q transport_id:2
emulator-5554          device product:sdk_gphone64_arm64 model:sdk_gphone64_arm64 transport_id:1
R58M123ABC             unauthorized usb:1-1 transport_id:3
0123456789             offline

`
	list := parseADB(out)
	if len(list) != 2 {
		t.Fatalf("got %d devices: %+v", len(list), list)
	}
	if d := list[0]; d.ID != "adb-5d9c2e71-k8wqzt._adb-tls-connect._tcp" || d.Name != "SM-S918B" || d.Emulator {
		t.Errorf("phone = %+v", d)
	}
	if d := list[1]; d.ID != "emulator-5554" || d.Name != "sdk gphone64 arm64" || !d.Emulator || d.TargetPlatform != "android" {
		t.Errorf("emulator = %+v", d)
	}
}
