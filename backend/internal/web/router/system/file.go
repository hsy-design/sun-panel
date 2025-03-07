package system

import (
	"fmt"
	"github.com/gin-gonic/gin/binding"
	"gorm.io/gorm"
	"net/http"
	"path"
	"strings"
	"sun-panel/internal/biz/repository"
	"sun-panel/internal/common"
	"sun-panel/internal/global"
	"sun-panel/internal/web/interceptor"
	"sun-panel/internal/web/model/base"
	"sun-panel/internal/web/model/param/commonApiStructs"
	"sun-panel/internal/web/model/response"
	"time"

	"github.com/gin-gonic/gin"
)

type FileApi struct {
	urlPrefix string
}

func NewFileRouter() *FileApi {
	return &FileApi{
		urlPrefix: global.Config.GetValueString("base", "url_prefix"),
	}
}

func (a *FileApi) InitRouter(router *gin.RouterGroup) {
	// 公共访问组，不需要 JWT 认证
	public := router.Group("")
	{
		// S3 文件访问路由
		public.GET("/file/s3/*filepath", a.GetS3File)
	}

	// 需要 JWT 认证的私有访问组
	private := router.Group("")
	private.Use(interceptor.JWTAuth)
	{
		private.POST("/file/uploadImg", a.UploadImg)
		private.POST("/file/deletes", a.Deletes)
		private.GET("/file/getList", a.GetList)
	}
}

func (a *FileApi) UploadImg(c *gin.Context) {
	userInfo, _ := base.GetCurrentUserInfo(c)
	f, err := c.FormFile("imgfile")
	if err != nil {
		response.ErrorByCode(c, 1300)
		return
	}

	fileExt := strings.ToLower(path.Ext(f.Filename))
	agreeExts := []string{
		".png",
		".jpg",
		".gif",
		".jpeg",
		".webp",
		".svg",
		".ico",
	}

	if !common.InArray(agreeExts, fileExt) {
		response.ErrorByCode(c, 1301)
		return
	}

	fileName := common.Md5(fmt.Sprintf("%s%s", f.Filename, time.Now().String())) + fileExt

	// 打开文件以获取Reader
	src, err := f.Open()
	if err != nil {
		global.Logger.Errorf("Failed to open uploaded file: %v", err)
		response.ErrorByCode(c, 1300)
		return
	}
	defer func() {
		if err := src.Close(); err != nil {
			global.Logger.Errorf("Failed to close file. error : %v", err)
		}
	}()

	// 使用存储接口上传文件
	filepath, err := global.Storage.Upload(c.Request.Context(), src, fileName)
	if err != nil {
		global.Logger.Errorf("Failed to upload file: %v", err)
		response.ErrorByCode(c, 1300)
		return
	}

	// 向数据库添加记录
	mFile := repository.File{}
	_, err = mFile.AddFile(userInfo.ID, f.Filename, fileExt, filepath)
	if err != nil {
		global.Logger.Errorf("Failed to add file record to database: %v", err)
		response.ErrorByCode(c, 1300)
		return
	}

	global.Logger.Infof("Successfully uploaded file %s to %s", f.Filename, filepath)
	response.SuccessData(c, gin.H{
		"imageUrl": a.urlPrefix + filepath,
	})
}

func (a *FileApi) GetList(c *gin.Context) {
	var list []repository.File
	userInfo, _ := base.GetCurrentUserInfo(c)
	var count int64
	if err := global.Db.Order("created_at desc").Find(&list, "user_id=?", userInfo.ID).Count(&count).Error; err != nil {
		response.ErrorDatabase(c, err.Error())
		return
	}

	var data []map[string]interface{}
	for _, v := range list {
		data = append(data, map[string]interface{}{
			"src":        a.urlPrefix + v.Src,
			"fileName":   v.FileName,
			"id":         v.ID,
			"createTime": v.CreatedAt,
			"updateTime": v.UpdatedAt,
		})
	}
	response.SuccessListData(c, data, count)
}

func (a *FileApi) Deletes(c *gin.Context) {
	req := commonApiStructs.RequestDeleteIds[uint]{}
	userInfo, _ := base.GetCurrentUserInfo(c)
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		response.ErrorParamFomat(c, err.Error())
		return
	}

	global.Db.Transaction(func(db *gorm.DB) error {
		var files []repository.File

		if err := db.Find(&files, "user_id=? AND id in ?", userInfo.ID, req.Ids).Order("created_at desc").Error; err != nil {
			return err
		}

		for _, v := range files {
			if err := global.Storage.Delete(c.Request.Context(), v.Src); err != nil {
				global.Logger.Errorf("Failed to delete file %s: %v", v.Src, err)
				return err
			}
		}

		if err := db.Delete(&files, "user_id=? AND id in ?", userInfo.ID, req.Ids).Error; err != nil {
			return err
		}

		return nil
	})

	response.Success(c)
}

func (a *FileApi) GetS3File(c *gin.Context) {
	global.Logger.Info("Entering GetS3File handler")
	global.Logger.Infof("Full URL Path: %s", c.Request.URL.Path)
	global.Logger.Infof("Full Request URL: %s", c.Request.URL.String())

	filepath := c.Param("filepath") // 获取 /api/file/s3/ 后的所有部分
	global.Logger.Infof("Extracted filepath: %s", filepath)

	if filepath == "" {
		global.Logger.Error("Empty filepath parameter")
		c.JSON(http.StatusBadRequest, gin.H{"error": "file path is required"})
		return
	}

	// 从存储中读取文件
	fileData, err := global.Storage.Get(c, filepath)
	if err != nil {
		global.Logger.Errorf("Failed to get file %s: %v", filepath, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get file"})
		return
	}

	// 设置文件类型
	contentType := "application/octet-stream"
	ext := path.Ext(filepath)
	switch ext {
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".png":
		contentType = "image/png"
	case ".gif":
		contentType = "image/gif"
	case ".svg":
		contentType = "image/svg+xml"
	}

	// 设置响应头
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", "inline; filename="+path.Base(filepath))

	global.Logger.Infof("Successfully serving file: %s with content type: %s", filepath, contentType)
	// 返回文件内容
	c.Data(http.StatusOK, contentType, fileData)
}
