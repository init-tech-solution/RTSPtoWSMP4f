package main

import (
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/format/mp4f"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/websocket"
)

func serveHTTP() {
	router := gin.Default()
	gin.SetMode(gin.DebugMode)
	router.LoadHTMLGlob("web/templates/*")
	router.GET("/", func(c *gin.Context) {
		all := streamSvc.listStreamUUIDs()
		sort.Strings(all)
		c.HTML(http.StatusOK, "index.tmpl", gin.H{
			"port":     streamSvc.Server.HTTPPort,
			"suuid":    all[0],
			"suuidMap": all,
			"version":  time.Now().String(),
		})
	})
	router.GET("/player/:suuid", func(c *gin.Context) {
		all := streamSvc.listStreamUUIDs()
		sort.Strings(all)
		c.HTML(http.StatusOK, "index.tmpl", gin.H{
			"port":     streamSvc.Server.HTTPPort,
			"suuid":    c.Param("suuid"),
			"suuidMap": all,
			"version":  time.Now().String(),
		})
	})
	router.GET("/ws/:suuid", func(c *gin.Context) {
		handler := websocket.Handler(ws)
		handler.ServeHTTP(c.Writer, c.Request)
	})
	router.StaticFS("/static", http.Dir("web/static"))
	err := router.Run(streamSvc.Server.HTTPPort)
	if err != nil {
		log.Fatalln(err)
	}
}
func ws(ws *websocket.Conn) {
	defer ws.Close()
	suuid := ws.Request().FormValue("suuid")
	log.Println("Request", suuid)

	if !streamSvc.isStreamAvail(suuid) {
		log.Println("Stream Not Found")
		return
	}

	streamSvc.RunIFNotRun(suuid)
	ws.SetWriteDeadline(time.Now().Add(5 * time.Second))

	generatedClientUUID, packetDataCh := streamSvc.clientRegisterStream(suuid)
	defer streamSvc.clientRemoveStream(suuid, generatedClientUUID)

	codecs := streamSvc.getStreamCodec(suuid)
	if codecs == nil {
		log.Println("Codecs Error")
		return
	}

	for i, codec := range codecs {
		if codec.Type().IsAudio() && codec.Type() != av.AAC {
			log.Println("Track", i, "Audio Codec Work Only AAC")
		}
	}

	muxer := mp4f.NewMuxer(nil)
	err := muxer.WriteHeader(codecs)
	if err != nil {
		log.Println("muxer.WriteHeader", err)
		return
	}
	// _, init := muxer.GetInit(codecs)
	meta, init := muxer.GetInit(codecs)
	log.Println(meta)

	// !TODO Send initial data
	// err = websocket.Message.Send(ws, append([]byte{9}, meta...))
	// if err != nil {
	// 	log.Println("websocket.Message.Send", err)
	// 	return
	// }

	err = websocket.Message.Send(ws, init)
	if err != nil {
		return
	}

	var start bool

	go func() {
		// Handle close channel
		for {
			var message string
			err := websocket.Message.Receive(ws, &message)
			if err != nil {
				ws.Close()
				return
			}
		}
	}()

	noVideoTimeOut := time.NewTimer(10 * time.Second)
	var timeLine = make(map[int8]time.Duration)

	for {
		select {
		case <-noVideoTimeOut.C:
			log.Println("noVideo")
			return

		case packetData := <-packetDataCh:
			if packetData.IsKeyFrame {
				noVideoTimeOut.Reset(10 * time.Second)
				start = true
			}

			if !start {
				continue
			}

			// log.Println("packetData.Idx", packetData.Idx)
			timeLine[packetData.Idx] = timeLine[packetData.Idx] + packetData.Duration
			packetData.Time = timeLine[packetData.Idx]

			ready, bufferData, _ := muxer.WritePacket(packetData, false)
			if ready {
				err = ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err != nil {
					return
				}

				err := websocket.Message.Send(ws, bufferData)
				if err != nil {
					return
				}
			}
		}
	}
}
