package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"sync"
	"time"

	"github.com/deepch/vdk/av"
)

//streamSvc global
var streamSvc = loadStreamSvcByConfig()

//StreamSvc struct
type StreamSvc struct {
	mutex   sync.RWMutex
	Server  ServerST            `json:"server"`
	Streams map[string]StreamST `json:"streams"`
}

//ServerST struct
type ServerST struct {
	HTTPPort string `json:"http_port"`
}

//StreamST struct
type StreamST struct {
	URL      string `json:"url"`
	Status   bool   `json:"status"`
	OnDemand bool   `json:"on_demand"`
	RunLock  bool   `json:"-"`
	Codecs   []av.CodecData
	Viewers  map[string]viewer
}

type viewer struct {
	packetChan chan av.Packet
}

func (streamSvc *StreamSvc) RunIFNotRun(uuid string) {
	streamSvc.mutex.Lock()
	defer streamSvc.mutex.Unlock()

	if tmp, ok := streamSvc.Streams[uuid]; ok {
		if tmp.OnDemand && !tmp.RunLock {
			tmp.RunLock = true
			streamSvc.Streams[uuid] = tmp
			go RTSPWorkerLoop(uuid, tmp.URL, tmp.OnDemand)
		}
	}
}

func (streamSvc *StreamSvc) RunUnlock(uuid string) {
	streamSvc.mutex.Lock()
	defer streamSvc.mutex.Unlock()
	if tmp, ok := streamSvc.Streams[uuid]; ok {
		if tmp.OnDemand && tmp.RunLock {
			tmp.RunLock = false
			streamSvc.Streams[uuid] = tmp
		}
	}
}

func (streamSvc *StreamSvc) HasViewer(uuid string) bool {
	streamSvc.mutex.Lock()
	defer streamSvc.mutex.Unlock()
	if tmp, ok := streamSvc.Streams[uuid]; ok && len(tmp.Viewers) > 0 {
		return true
	}
	return false
}

func loadStreamSvcByConfig() *StreamSvc {
	var tmp StreamSvc
	data, err := ioutil.ReadFile("config.json")
	if err != nil {
		log.Fatalln(err)
	}
	err = json.Unmarshal(data, &tmp)
	if err != nil {
		log.Fatalln(err)
	}
	for i, v := range tmp.Streams {
		v.Viewers = make(map[string]viewer)
		tmp.Streams[i] = v
	}
	return &tmp
}

func (streamSvc *StreamSvc) sendPacketDataToStream(suuid string, packetData av.Packet) {
	streamSvc.mutex.Lock()
	defer streamSvc.mutex.Unlock()
	for _, v := range streamSvc.Streams[suuid].Viewers {
		if len(v.packetChan) < cap(v.packetChan) {
			v.packetChan <- packetData
		}
	}
}

func (streamSvc *StreamSvc) isStreamAvail(suuid string) bool {
	streamSvc.mutex.Lock()
	defer streamSvc.mutex.Unlock()
	_, ok := streamSvc.Streams[suuid]
	return ok
}

func (streamSvc *StreamSvc) addCodecToStreamByUUID(suuid string, codecs []av.CodecData) {
	streamSvc.mutex.Lock()
	defer streamSvc.mutex.Unlock()
	t := streamSvc.Streams[suuid]
	t.Codecs = codecs
	streamSvc.Streams[suuid] = t
}

func (streamSvc *StreamSvc) getStreamCodec(suuid string) []av.CodecData {
	for i := 0; i < 100; i++ {
		streamSvc.mutex.RLock()
		tmp, ok := streamSvc.Streams[suuid]
		streamSvc.mutex.RUnlock()
		if !ok {
			return nil
		}
		if tmp.Codecs != nil {
			return tmp.Codecs
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

func (streamSvc *StreamSvc) clientRegisterStream(suuid string) (string, chan av.Packet) {
	streamSvc.mutex.Lock()
	defer streamSvc.mutex.Unlock()
	clientUUID := genClientUUID()
	ch := make(chan av.Packet, 100)
	streamSvc.Streams[suuid].Viewers[clientUUID] = viewer{packetChan: ch}
	return clientUUID, ch
}

func (streamSvc *StreamSvc) listStreamUUIDs() []string {
	streamSvc.mutex.Lock()
	defer streamSvc.mutex.Unlock()
	var res []string
	for k := range streamSvc.Streams {

		res = append(res, k)
	}
	return res
}
func (streamSvc *StreamSvc) clientRemoveStream(suuid, cuuid string) {
	streamSvc.mutex.Lock()
	defer streamSvc.mutex.Unlock()
	delete(streamSvc.Streams[suuid].Viewers, cuuid)
}

func genClientUUID() (uuid string) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	uuid = fmt.Sprintf("%X-%X-%X-%X-%X", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
	return
}
