package app

import (
	"net"
	"testing"
)

func TestFakeIPIsNotASourceAddress(t *testing.T) {
	if routableIP(net.ParseIP("198.18.2.97")) || routableIP(net.ParseIP("192.168.1.1")) || routableIP(net.ParseIP("127.0.0.1")) {
		t.Fatal("proxy and private addresses are not where the source is")
	}
	if !routableIP(net.ParseIP("180.101.51.73")) {
		t.Fatal("public address should be kept")
	}
	body := []byte(`{"Status":0,"Answer":[{"name":"www.baidu.com.","type":5,"data":"www.a.shifen.com."},{"name":"www.a.shifen.com.","type":1,"data":"198.18.0.4"},{"name":"www.a.shifen.com.","type":1,"data":"180.101.51.73"}]}`)
	if got := firstAnswerIP(body); got != "180.101.51.73" {
		t.Fatalf("answer %q", got)
	}
}
