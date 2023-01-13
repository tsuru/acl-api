// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mongodb

import (
	"context"
	"sync"
	"time"

	"github.com/tsuru/acl-api/api/types"
	"github.com/tsuru/acl-api/storage"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var lockExpireTime = 5 * time.Minute

var syncOnce sync.Once

type syncStorage struct {
	*mongoStorage
}

var _ storage.SyncStorage = &syncStorage{}

func (s *syncStorage) getSyncColl() *mongo.Collection {
	coll := s.getCollection("acl_rule_sync")
	syncOnce.Do(func() {
		coll.Indexes().CreateOne(context.TODO(), mongo.IndexModel{
			Keys: bson.D{
				{Key: "ruleid", Value: 1},
				{Key: "engine", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		})
		coll.Indexes().CreateOne(context.TODO(), mongo.IndexModel{
			Keys: bson.D{
				{Key: "starttime", Value: -1},
			},
		})
	})
	return coll
}

func (s *syncStorage) SetLockExpireTime(timeout time.Duration) time.Duration {
	oldLockExpireTime := lockExpireTime
	lockExpireTime = timeout
	return oldLockExpireTime
}

type ruleSyncInfo struct {
	SyncID    string `bson:"_id,omitempty"`
	RuleID    string
	Engine    string
	StartTime time.Time
	EndTime   time.Time
	PingTime  time.Time
	Running   bool
	Syncs     []types.RuleSyncData
}

func (s *syncStorage) StartSync(after time.Duration, ruleID, engine string, force bool) (time.Duration, *types.RuleSyncInfo, error) {
	coll := s.getSyncColl()
	expireTime := lockExpireTime
	if after > expireTime {
		expireTime = after
	}
	now := time.Now().UTC()

	query := bson.M{
		"ruleid": ruleID,
		"engine": engine,
	}

	if !force {
		query["$or"] = []bson.M{
			{
				"pingtime": bson.M{"$lt": now.Add(-after)},
				"running":  false,
			},
			{
				"pingtime": bson.M{"$lt": now.Add(-expireTime)},
				"running":  true,
			},
		}
	}

	result := coll.FindOneAndUpdate(context.TODO(), query, bson.M{
		"$setOnInsert": bson.M{
			"_id": newID(),
		},
		"$set": bson.M{
			"ruleid":    ruleID,
			"engine":    engine,
			"starttime": now,
			"pingtime":  now,
			"running":   true,
		},
	}, options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After))

	next := after
	err := result.Err()
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			err = storage.ErrSyncStorageLocked
			var findResult ruleSyncInfo
			coll.FindOne(context.TODO(), bson.M{
				"ruleid":  ruleID,
				"engine":  engine,
				"running": false,
			}).Decode(&findResult)
			if !findResult.PingTime.IsZero() {
				next = after - time.Now().UTC().Sub(findResult.PingTime)
			}
		}
		return next, nil, err
	}
	var syncResult ruleSyncInfo
	err = result.Decode(&syncResult)
	if err != nil {
		return next, nil, err
	}
	ruleSync := types.RuleSyncInfo(syncResult)
	return next, &ruleSync, err
}

func (s *syncStorage) PingSyncs(ruleSyncIDs []string) error {
	coll := s.getSyncColl()
	_, err := coll.UpdateMany(context.TODO(), bson.M{
		"_id": bson.M{"$in": ruleSyncIDs},
	}, bson.M{"$set": bson.M{"pingtime": time.Now().UTC()}})
	return err
}

func (s *syncStorage) EndSync(ruleSync types.RuleSyncInfo, syncData types.RuleSyncData) error {
	coll := s.getSyncColl()
	now := time.Now().UTC()
	_, err := coll.UpdateOne(context.TODO(), bson.M{
		"ruleid": ruleSync.RuleID,
		"engine": ruleSync.Engine,
	}, bson.M{
		"$set": bson.M{
			"running":  false,
			"pingtime": now,
			"endtime":  now,
		},
		"$push": bson.M{
			"syncs": bson.D{
				{Key: "$each", Value: []types.RuleSyncData{syncData}},
				{Key: "$slice", Value: -10},
			},
		},
	})
	return err
}

func (s *syncStorage) Find(opts storage.SyncFindOpts) ([]types.RuleSyncInfo, error) {
	coll := s.getSyncColl()
	filter := bson.M{}
	if opts.Engines != nil {
		filter["engine"] = bson.M{"$in": opts.Engines}
	}
	if opts.RuleIDs != nil {
		filter["ruleid"] = bson.M{"$in": opts.RuleIDs}
	}
	findOpts := options.Find().SetSort(bson.M{"starttime": -1})
	if opts.Limit > 0 {
		findOpts = findOpts.SetLimit(int64(opts.Limit))
	}
	cur, err := coll.Find(context.TODO(), filter, findOpts)
	if err != nil {
		return nil, err
	}
	var rawSyncInfos []ruleSyncInfo
	err = cur.All(context.TODO(), &rawSyncInfos)
	if err != nil {
		return nil, err
	}
	syncInfos := make([]types.RuleSyncInfo, len(rawSyncInfos))
	for i := range rawSyncInfos {
		syncInfos[i] = types.RuleSyncInfo(rawSyncInfos[i])
	}
	return syncInfos, nil
}
