package system

import (
	"sun-panel/api/api_v1/common/apiReturn"
	"sun-panel/lib/cmn"

	"github.com/gin-gonic/gin"
)

type About struct {
}

func (a *About) Get(c *gin.Context) {
	version := cmn.GetVersion()
	apiReturn.SuccessData(c, gin.H{
		"versionName": version,
	})
}
