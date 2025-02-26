package router

import (
	"sun-panel/global"
	"sun-panel/router/panel"
	"sun-panel/router/system"

	"github.com/gin-gonic/gin"
)

func InitRouters(addr string) error {
	router := gin.Default()
	rootRouter := router.Group("/")
	routerGroup := rootRouter.Group("api")

	// 接口
	system.Init(routerGroup)
	panel.Init(routerGroup)

	// WEB文件服务
	if global.Config.GetValueString("base", "enable_static_server") == "true" {
		webPath := "./web"
		router.Static("/assets", webPath+"/assets")
		router.Static("/custom", webPath+"/custom")
		router.StaticFile("/", webPath+"/index.html")
		router.StaticFile("/favicon.ico", webPath+"/favicon.ico")
		router.StaticFile("/favicon.svg", webPath+"/favicon.svg")

		if global.Config.GetValueString("rclone", "type") == "local" {
			// 使用本次存储时，为本次存储设置静态文件服务
			sourcePath := global.Config.GetValueString("base", "source_path")
			router.Static("/"+sourcePath, sourcePath)
		}

		global.Logger.Info("Static file server is enabled")
	} else {
		global.Logger.Info("Static file server is disabled")
	}

	global.Logger.Info("Sun-Panel is Started.  Listening and serving HTTP on ", addr)
	return router.Run(addr)
}
