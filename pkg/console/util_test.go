package console

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/harvester/harvester-installer/pkg/util"
)

func TestGetSSHKeysFromURL(t *testing.T) {
	testCases := []struct {
		name         string
		httpResp     string
		pubKeysCount int
		expectError  string
	}{
		{
			name:         "Two public keys",
			httpResp:     string(util.LoadFixture(t, "keys")),
			pubKeysCount: 2,
		},
		{
			name:        "Invalid public key",
			httpResp:    "\nooxx",
			expectError: "fail to parse on line 2: ooxx",
		},
		{
			name:        "No public key",
			httpResp:    "",
			expectError: "no key found",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprintln(w, testCase.httpResp)
			}))
			defer ts.Close()

			pubKeys, err := getRemoteSSHKeys(ts.URL)
			if testCase.expectError != "" {
				assert.EqualError(t, err, testCase.expectError)
			} else {
				assert.Equal(t, nil, err)
				assert.Equal(t, testCase.pubKeysCount, len(pubKeys))
			}
		})
	}
}

func TestGetHStatus(t *testing.T) {
	s := getHarvesterStatus()
	t.Log(s)
}

func TestGetFormattedServerURL(t *testing.T) {
	testCases := []struct {
		Name   string
		input  string
		output string
		err    error
	}{
		{
			Name:   "ip",
			input:  "1.2.3.4",
			output: "https://1.2.3.4:443",
			err:    nil,
		},
		{
			Name:   "domain name",
			input:  "example.org",
			output: "https://example.org:443",
			err:    nil,
		},
		{
			Name:   "ip without port and scheme",
			input:  "1.1.1.1",
			output: "https://1.1.1.1:443",
			err:    nil,
		},
		{
			Name:   "domain without port and scheme",
			input:  "abc.org",
			output: "https://abc.org:443",
			err:    nil,
		},
		{
			Name:   "custom port",
			input:  "1.2.3.4:555",
			output: "",
			err:    errors.New("currently non-443 port are not allowed"),
		},
		{
			Name:   "ip with path",
			input:  "1.2.3.4/",
			output: "",
			err:    errors.New("path is not allowed in management address: /"),
		},
		{
			Name:   "domain with path",
			input:  "abc.org/test/abc",
			output: "",
			err:    errors.New("path is not allowed in management address: /test/abc"),
		},
	}
	for _, testCase := range testCases {
		got, err := getFormattedServerURL(testCase.input)
		assert.Equal(t, testCase.output, got)
		assert.Equal(t, testCase.err, err)
	}
}

func TestF(t *testing.T) {
	ifaces, _ := net.Interfaces()
	for _, i := range ifaces {
		addrs, _ := i.Addrs()
		// handle err
		for _, addr := range addrs {
			if v, ok := addr.(*net.IPNet); ok && !v.IP.IsLoopback() && v.IP.To4() != nil {
				t.Log(v.IP.String())
			}
		}
	}
}

func TestGetServerURLFromRancherdConfig(t *testing.T) {
	testCases := []struct {
		input []byte
		url   string
		err   error
	}{
		{
			input: []byte("role: cluster-init\nkubernetesVersion: v1.21.2+rke2r1"),
			url:   "",
			err:   nil,
		},
		{
			input: []byte("role: agent\nkubernetesVersion: v1.21.2+rke2r1\nserver: https://172.0.0.1:443"),
			url:   "https://172.0.0.1:443",
			err:   nil,
		},
	}

	for _, testCase := range testCases {
		url, err := getServerURLFromRancherdConfig(testCase.input)
		assert.Equal(t, testCase.url, url)
		assert.Equal(t, testCase.err, err)
	}
}

func TestValidateNTPServers(t *testing.T) {
	quit := make(chan interface{})
	mockNTPServers, err := startMockNTPServers(quit)
	if err != nil {
		t.Fatalf("can't start mock ntp servers, %v", err)
	}
	testCases := []struct {
		name        string
		input       []string
		expectError bool
	}{
		{
			name:        "Correct NTP Servers",
			input:       mockNTPServers,
			expectError: false,
		},
		{
			name:        "Empty input",
			input:       []string{},
			expectError: false,
		},
		{
			name:        "Invalid URL",
			input:       []string{"error"},
			expectError: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := validateNTPServers(testCase.input)
			if testCase.expectError {
				assert.NotNil(t, err)
			} else {
				if err != nil {
					t.Log(err)
				}
				assert.Nil(t, err)
			}
		})
	}
	close(quit)
}

func startMockNTPServers(quit chan interface{}) ([]string, error) {
	ntpServers := []string{}
	for i := 0; i < 2; i++ {
		listener, err := net.ListenPacket("udp", "127.0.0.1:0")
		if err != nil {
			return nil, err
		}
		ntpServers = append(ntpServers, listener.LocalAddr().String())

		go func(listener net.PacketConn) {
			defer listener.Close()

			for {
				req := make([]byte, 48)
				_, addr, err := listener.ReadFrom(req)
				if err != nil {
					select {
					case <-quit:
						return
					default:
						continue
					}
				}
				go func(listener net.PacketConn, addr net.Addr) {
					listener.WriteTo(make([]byte, 48), addr)
				}(listener, addr)
			}

		}(listener)
	}
	return ntpServers, nil
}

const (
	sampleSerialDiskOutput = `
{
   "blockdevices": [
      {
         "name": "loop0",
         "size": "768.1M",
         "type": "loop",
         "wwn": null,
         "serial": null
      },{
         "name": "sda",
         "size": "250G",
         "type": "disk",
         "wwn": null,
         "serial": "serial-1",
         "children": [
            {
               "name": "0QEMU_QEMU_HARDDISK_serial-1",
               "size": "250G",
               "type": "mpath",
               "wwn": null,
               "serial": null
            }
         ]
      },{
         "name": "sdb",
         "size": "250G",
         "type": "disk",
         "wwn": null,
         "serial": "serial-1",
         "children": [
            {
               "name": "0QEMU_QEMU_HARDDISK_serial-1",
               "size": "250G",
               "type": "mpath",
               "wwn": null,
               "serial": null
            }
         ]
      },{
         "name": "sr0",
         "size": "5.8G",
         "type": "rom",
         "wwn": null,
         "serial": "QM00001"
      }
   ]
}
`

	reinstallDisks = `
{
   "blockdevices": [
      {
         "name": "loop0",
         "size": "3G",
         "type": "loop",
         "wwn": null,
         "serial": null
      },{
         "name": "loop1",
         "size": "10G",
         "type": "loop",
         "wwn": null,
         "serial": null
      },{
         "name": "sda",
         "size": "10G",
         "type": "disk",
         "wwn": "0x60000000000000000e00000000010001",
         "serial": "beaf11",
         "children": [
            {
               "name": "sda1",
               "size": "2.5G",
               "type": "part",
               "wwn": "0x60000000000000000e00000000010001",
               "serial": null
            },{
               "name": "sda14",
               "size": "4M",
               "type": "part",
               "wwn": "0x60000000000000000e00000000010001",
               "serial": null
            },{
               "name": "sda15",
               "size": "106M",
               "type": "part",
               "wwn": "0x60000000000000000e00000000010001",
               "serial": null
            },{
               "name": "sda16",
               "size": "913M",
               "type": "part",
               "wwn": "0x60000000000000000e00000000010001",
               "serial": null
            }
         ]
      },{
         "name": "sr0",
         "size": "364K",
         "type": "rom",
         "wwn": null,
         "serial": "QM00001"
      },{
         "name": "vda",
         "size": "250G",
         "type": "disk",
         "wwn": null,
         "serial": null,
         "children": [
            {
               "name": "vda1",
               "size": "1M",
               "type": "part",
               "wwn": null,
               "serial": null
            },{
               "name": "vda2",
               "size": "50M",
               "type": "part",
               "wwn": null,
               "serial": null
            },{
               "name": "vda3",
               "size": "8G",
               "type": "part",
               "wwn": null,
               "serial": null
            },{
               "name": "vda4",
               "size": "15G",
               "type": "part",
               "wwn": null,
               "serial": null
            },{
               "name": "vda5",
               "size": "150G",
               "type": "part",
               "wwn": null,
               "serial": null
            },{
               "name": "vda6",
               "size": "76.9G",
               "type": "part",
               "wwn": null,
               "serial": null
            }
         ]
      }
   ]
}
`

	preInstalledMultiPath = `
{
   "blockdevices": [
      {
         "name": "loop0",
         "size": "768.4M",
         "type": "loop",
         "wwn": null,
         "serial": null
      },{
         "name": "sda",
         "size": "250G",
         "type": "disk",
         "wwn": null,
         "serial": "disk1",
         "children": [
            {
               "name": "sda1",
               "size": "1M",
               "type": "part",
               "wwn": null,
               "serial": null
            },{
               "name": "sda2",
               "size": "50M",
               "type": "part",
               "wwn": null,
               "serial": null
            },{
               "name": "sda3",
               "size": "8G",
               "type": "part",
               "wwn": null,
               "serial": null
            },{
               "name": "sda4",
               "size": "15G",
               "type": "part",
               "wwn": null,
               "serial": null
            },{
               "name": "sda5",
               "size": "150G",
               "type": "part",
               "wwn": null,
               "serial": null
            },{
               "name": "sda6",
               "size": "76.9G",
               "type": "part",
               "wwn": null,
               "serial": null
            },{
               "name": "0QEMU_QEMU_HARDDISK_disk1",
               "size": "250G",
               "type": "mpath",
               "wwn": null,
               "serial": null,
               "children": [
                  {
                     "name": "0QEMU_QEMU_HARDDISK_disk1-part1",
                     "size": "1M",
                     "type": "part",
                     "wwn": null,
                     "serial": null
                  },{
                     "name": "0QEMU_QEMU_HARDDISK_disk1-part2",
                     "size": "50M",
                     "type": "part",
                     "wwn": null,
                     "serial": null
                  },{
                     "name": "0QEMU_QEMU_HARDDISK_disk1-part3",
                     "size": "8G",
                     "type": "part",
                     "wwn": null,
                     "serial": null
                  },{
                     "name": "0QEMU_QEMU_HARDDISK_disk1-part4",
                     "size": "15G",
                     "type": "part",
                     "wwn": null,
                     "serial": null
                  },{
                     "name": "0QEMU_QEMU_HARDDISK_disk1-part5",
                     "size": "150G",
                     "type": "part",
                     "wwn": null,
                     "serial": null
                  },{
                     "name": "0QEMU_QEMU_HARDDISK_disk1-part6",
                     "size": "76.9G",
                     "type": "part",
                     "wwn": null,
                     "serial": null
                  }
               ]
            }
         ]
      },{
         "name": "sdb",
         "size": "250G",
         "type": "disk",
         "wwn": null,
         "serial": "disk1",
         "children": [
            {
               "name": "sdb1",
               "size": "1M",
               "type": "part",
               "wwn": null,
               "serial": null
            },{
               "name": "sdb2",
               "size": "50M",
               "type": "part",
               "wwn": null,
               "serial": null
            },{
               "name": "sdb3",
               "size": "8G",
               "type": "part",
               "wwn": null,
               "serial": null
            },{
               "name": "sdb4",
               "size": "15G",
               "type": "part",
               "wwn": null,
               "serial": null
            },{
               "name": "sdb5",
               "size": "150G",
               "type": "part",
               "wwn": null,
               "serial": null
            },{
               "name": "sdb6",
               "size": "76.9G",
               "type": "part",
               "wwn": null,
               "serial": null
            },{
               "name": "0QEMU_QEMU_HARDDISK_disk1",
               "size": "250G",
               "type": "mpath",
               "wwn": null,
               "serial": null,
               "children": [
                  {
                     "name": "0QEMU_QEMU_HARDDISK_disk1-part1",
                     "size": "1M",
                     "type": "part",
                     "wwn": null,
                     "serial": null
                  },{
                     "name": "0QEMU_QEMU_HARDDISK_disk1-part2",
                     "size": "50M",
                     "type": "part",
                     "wwn": null,
                     "serial": null
                  },{
                     "name": "0QEMU_QEMU_HARDDISK_disk1-part3",
                     "size": "8G",
                     "type": "part",
                     "wwn": null,
                     "serial": null
                  },{
                     "name": "0QEMU_QEMU_HARDDISK_disk1-part4",
                     "size": "15G",
                     "type": "part",
                     "wwn": null,
                     "serial": null
                  },{
                     "name": "0QEMU_QEMU_HARDDISK_disk1-part5",
                     "size": "150G",
                     "type": "part",
                     "wwn": null,
                     "serial": null
                  },{
                     "name": "0QEMU_QEMU_HARDDISK_disk1-part6",
                     "size": "76.9G",
                     "type": "part",
                     "wwn": null,
                     "serial": null
                  }
               ]
            }
         ]
      },{
         "name": "sr0",
         "size": "5.8G",
         "type": "rom",
         "wwn": null,
         "serial": "QM00001"
      }
   ]
}
`
)

func Test_identifyUniqueDisksWithSerialNumber(t *testing.T) {
	assert := require.New(t)
	result, err := identifyUniqueDisks([]byte(sampleSerialDiskOutput))
	assert.NoError(err, "expected no error while parsing disk data")
	assert.Len(result, 1, "expected to find 1 disk only")
}

func Test_identifyUniqueDisksWithExistingData(t *testing.T) {
	assert := require.New(t)
	result, err := identifyUniqueDisks([]byte(reinstallDisks))
	assert.NoError(err, "expected no error while parsing disk data")
	assert.Len(result, 2, "expected to find 2 disks only")
}

func Test_identifyUniqueDisksOnExistingInstalls(t *testing.T) {
	assert := require.New(t)
	result, err := identifyUniqueDisks([]byte(preInstalledMultiPath))
	assert.NoError(err, "expected no error while parsing disk data")
	assert.Len(result, 1, "expected to find 1 disk only")
}
