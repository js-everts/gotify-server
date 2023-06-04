package ui

import (
	"bytes"
	"embed"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/gotify/server/v2/model"
)

//go:embed build
var uiFS embed.FS

type uiConfig struct {
	Register bool              `json:"register"`
	Version  model.VersionInfo `json:"version"`
}

type handler struct {
	fs           http.FileSystem
	indexModTime string
	indexBytes   []byte
}

func (h *handler) serveIndex(ctx *gin.Context) {
	ctx.Header("Last-Modified", h.indexModTime)

	ctx.Data(http.StatusOK, "text/html", h.indexBytes)
}

func (h *handler) serveOther(ctx *gin.Context) {
	// the error will either be IsNotExist error or some path related error
	// which we can treat as not found
	file, err := h.fs.Open(ctx.Request.URL.Path)
	if err != nil {
		ctx.AbortWithStatus(404)
		return
	}

	// Stat from embed.FS will never error
	s, _ := file.Stat()
	if s.IsDir() {
		ctx.AbortWithStatus(404)
		return
	}

	// ServeContent will set the correct content-type header
	http.ServeContent(ctx.Writer, ctx.Request, s.Name(), s.ModTime(), file)
}

func createHandler(uiConfigBytes []byte) *handler {
	subFS, err := fs.Sub(uiFS, "build")
	if err != nil {
		panic(err)
	}

	// will only return an error if index.html doesn't exist and will cause
	// io.ReadAll to panic.
	file, _ := subFS.Open("index.html")
	idxBytes, _ := io.ReadAll(file)
	stat, _ := file.Stat()

	idxBytes = bytes.Replace(idxBytes, []byte("%CONFIG%"), uiConfigBytes, 1)

	return &handler{
		indexBytes:   idxBytes,
		indexModTime: stat.ModTime().UTC().Format(http.TimeFormat),
		fs:           http.FS(subFS),
	}
}

// Register registers the ui on the root path.
func Register(r *gin.Engine, version model.VersionInfo, register bool) {
	uiConfigBytes, err := json.Marshal(uiConfig{Version: version, Register: register})
	if err != nil {
		panic(err)
	}

	h := createHandler(uiConfigBytes)

	ui := r.Group("/", gzip.Gzip(gzip.DefaultCompression))
	ui.GET("/", h.serveIndex)
	ui.GET("/index.html", h.serveIndex)

	ui.GET("/manifest.json", h.serveOther)
	ui.GET("/asset-manifest.json", h.serveOther)
	ui.GET("/static/*any", h.serveOther)
}
