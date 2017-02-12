package controllers

import (
	"log"
	"strconv"
	"strings"

	"github.com/pclubiitk"

	"github.com/kataras/iris"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// @AUTH @Admin Drop compute table
// ----------------------------------------------------
func Serve(ctx *iris.Context) {
	id, err := SessionId(ctx)
	if err != nil || id != "admin" {
		ctx.EmitError(iris.StatusForbidden)
		return
	}

	if err := m.Db.GetCollection("compute").DropCollection(); err != nil {
		ctx.Text(iris.StatusInternalServerError, "Could not delete collection")
		return
	}

	ctx.JSON(iris.StatusOK, "Deleted compute table")
}

// @AUTH @Admin Create the entries in the compute table
// ----------------------------------------------------

func Serve(ctx *iris.Context) {
	id, err := SessionId(ctx)
	if err != nil || id != "admin" {
		ctx.EmitError(iris.StatusForbidden)
		return
	}

	type typeIds struct {
		Id string `json:"_id" bson:"_id"`
	}

	var females []typeIds
	var males []typeIds

	collection := m.Db.GetCollection("user")
	err1 := collection.Find(bson.M{"gender": "0"}).All(&females)
	err2 := collection.Find(bson.M{"gender": "1"}).All(&males)

	if err1 != nil || err2 != nil {
		ctx.EmitError(iris.StatusInternalServerError)
		return
	}

	cnt := 0

	// Whether to enable experimental bulk calls
	experimentalBulk := ctx.Param("bulk")

	if experimentalBulk == "1" {
		bulk := m.Db.GetCollection("compute").Bulk()
		bulk.Unordered()
		for _, fe := range females {
			for _, ma := range males {
				log.Println(fe.Id, "-", ma.Id, "-", cnt)
				cnt = cnt + 1
				res := models.UpsertEntry(fe.Id, ma.Id)
				bulk.Upsert(res.Selector, res.Change)
			}
		}
		r, err := bulk.Run()
		if err != nil {
			ctx.Error("Something failed", iris.StatusInternalServerError)
			log.Println("Bulk call failed")
			log.Println(err)
			return
		}
		ctx.JSON(iris.StatusOK, r)
	} else {
		compute_coll := m.Db.GetCollection("compute")
		for _, fe := range females {
			for _, ma := range males {
				log.Println(fe.Id, "-", ma.Id, "-", cnt)
				cnt = cnt + 1
				res := models.UpsertEntry(fe.Id, ma.Id)
				compute_coll.Upsert(res.Selector, res.Change)
			}
		}
		ctx.JSON(iris.StatusOK, strconv.Itoa(cnt)+" entries created!")
	}
}

// @AUTH @Admin Create the entries in the compute table for given user
// -------------------------------------------------------------------
func Serve(ctx *iris.Context) {
	id, err := SessionId(ctx)
	if err != nil || id != "admin" {
		ctx.EmitError(iris.StatusForbidden)
		return
	}

	type typeIds struct {
		Id string `json:"_id" bson:"_id"`
	}

	var people []typeIds

	uid := ctx.Param("id")
	req_gender := ctx.Param("gender")
	if id == "" || req_gender == "" {
		ctx.Error("Id or gender not provided /:id/:gender", iris.StatusBadRequest)
		return
	}

	collection := m.Db.GetCollection("user")
	err = collection.Find(bson.M{"gender": req_gender}).All(&people)

	if err != nil {
		ctx.EmitError(iris.StatusInternalServerError)
		log.Println(err)
		return
	}

	cnt := 0
	compute_coll := m.Db.GetCollection("compute")
	for _, fe := range people {
		log.Println(fe.Id, "-", uid, "-", cnt)
		cnt = cnt + 1
		res := models.UpsertEntry(fe.Id, uid)
		compute_coll.Upsert(res.Selector, res.Change)
	}
	ctx.JSON(iris.StatusOK, strconv.Itoa(cnt)+" entries created!")
}

func Serve(ctx *iris.Context) {
	id, err := SessionId(ctx)
	if err != nil {
		ctx.EmitError(iris.StatusForbidden)
		return
	}

	// Depending on the thing to update, set the needed variables
	var dbUpdate string
	if m.State == 0 {
		dbUpdate = "t"
	} else {
		log.Print("Something seems wrong here: ", m.State)
		ctx.Error("Wrong state code", iris.StatusBadRequest)
		return
	}

	user := struct {
		State int32 `json:"state" bson:"state"`
	}{}

	// Check that the user is valid
	if err := m.Db.GetById("user", id).One(&user); err != nil {
		ctx.JSON(iris.StatusBadRequest, "Invalid user")
		return
	}

	// Verify valid requested changes
	info := new([]idToken)
	if err := ctx.ReadJSON(info); err != nil {
		ctx.JSON(iris.StatusBadRequest, "Invalid JSON")
		log.Println(err)
		return
	}

	// Verify all ids are valid
	for _, pInfo := range *info {
		if !models.CheckId(pInfo.Id, id) {
			ctx.JSON(iris.StatusBadRequest, "Invalid ID")
			return
		}
	}

	// Bulk update all entries
	// dbUpdate+"1" means t1 in case of tokens, r1 in case of results, v1 too
	bulk := m.Db.GetCollection("compute").Bulk()

	bulk.Unordered()

	var chunks []string
	var update bson.M

	cnt := 0
	res := []*mgo.BulkResult{}
	for _, pInfo := range *info {
		chunks = strings.Split(pInfo.Id, "-")

		if chunks[0] == id {
			update = bson.M{"$set": bson.M{dbUpdate + "0": pInfo.Value}}
		} else {
			update = bson.M{"$set": bson.M{dbUpdate + "1": pInfo.Value}}
		}

		bulk.Update(bson.M{"_id": pInfo.Id}, update)
		cnt = cnt + 1

		if cnt > 950 {
			r, err := bulk.Run()
			bulk = m.Db.GetCollection("compute").Bulk()
			bulk.Unordered()
			res = append(res, r)
			if err != nil {
				log.Println("ERROR: Bulk seems broken")
				log.Println(err)
			}
			cnt = 0
		}
	}

	r, err := bulk.Run()
	res = append(res, r)
	if err != nil {
		log.Println("ERROR: Bulk seems broken")
		log.Println(err)
	}

	ctx.JSON(iris.StatusOK, res)
}
