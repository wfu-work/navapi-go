package apis

import (
	"navapi-go/domains"
	"navapi-go/services"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
)

type ModelApi struct{}

func (a ModelApi) List(c *gin.Context) {
	models, err := services.ModelServiceApp.ListMeta()
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(models, c)
}

func (a ModelApi) Upsert(c *gin.Context) {
	var meta domains.ModelMeta
	if err := c.ShouldBindJSON(&meta); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := services.ModelServiceApp.UpsertMeta(&meta); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(meta, c)
}

func (a ModelApi) Delete(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := services.ModelServiceApp.DeleteMeta(id); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}

func (a ModelApi) PublicVendors(c *gin.Context) {
	vendors, err := services.ModelServiceApp.ListVendors(false)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(vendors, c)
}

func (a ModelApi) Vendors(c *gin.Context) {
	vendors, err := services.ModelServiceApp.ListVendors(true)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(vendors, c)
}

func (a ModelApi) UpsertVendor(c *gin.Context) {
	var meta domains.VendorMeta
	if err := c.ShouldBindJSON(&meta); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := services.ModelServiceApp.UpsertVendor(&meta); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(meta, c)
}

func (a ModelApi) DeleteVendor(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := services.ModelServiceApp.DeleteVendor(id); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}
