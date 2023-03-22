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

var (
	_ storage.RuleStorage    = &ruleStorage{}
	_ storage.ServiceStorage = &serviceStorage{}
)

var ruleOnce sync.Once

// rule struct must be kept in sync with types.Rule
type rule struct {
	RuleID      string `bson:"_id"`
	RuleName    string `bson:"name,omitempty"`
	Source      types.RuleType
	Destination types.RuleType
	Removed     bool
	Metadata    map[string]string
	Created     time.Time
	Creator     string
}

type ruleStorage struct {
	*mongoStorage
}

func (s *ruleStorage) getRulesColl() *mongo.Collection {
	coll := s.getCollection("acl_rules")

	ruleOnce.Do(func() {
		coll.Indexes().CreateOne(context.TODO(), mongo.IndexModel{
			Keys: bson.D{
				{Key: "name", Value: 1},
			},
			Options: options.Index().SetUnique(true).SetSparse(true),
		})

		coll.Indexes().CreateOne(context.TODO(), mongo.IndexModel{
			Keys: bson.D{
				{Key: "source.tsuruapp.appname", Value: 1},
			},
			Options: options.Index(),
		})

		coll.Indexes().CreateOne(context.TODO(), mongo.IndexModel{
			Keys: bson.D{
				{Key: "source.tsurujob.jobname", Value: 1},
			},
			Options: options.Index(),
		})
	})

	return coll
}

func (s *ruleStorage) Find(id string) (types.Rule, error) {
	coll := s.getRulesColl()
	result := coll.FindOne(context.TODO(), bson.M{"$or": bson.A{
		bson.M{"_id": id},
		bson.M{"name": id},
	}})

	err := result.Err()
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return types.Rule{}, storage.ErrRuleNotFound
		}
		return types.Rule{}, err
	}
	var r rule
	err = result.Decode(&r)
	if err != nil {
		return types.Rule{}, err
	}
	return types.Rule(r), nil
}

func (s *ruleStorage) Save(rules []*types.Rule, upsert bool) error {
	ctx := context.TODO()
	now := time.Now().UTC()
	for _, r := range rules {
		if r.RuleID == "" {
			r.RuleID = newID()
		}
		r.Created = now
	}
	coll := s.getRulesColl()
	var err error
	if !upsert {
		var toInsert []interface{}
		for _, r := range rules {
			toInsert = append(toInsert, rule(*r))
		}
		_, err = coll.InsertMany(ctx, toInsert)

		if err != nil {
			if mongo.IsDuplicateKeyError(err) {
				return storage.ErrInstanceAlreadyExists
			}
			return err
		}
		return nil
	}
	for _, r := range rules {
		_, err = coll.ReplaceOne(ctx, bson.M{"_id": r.RuleID}, r, &options.ReplaceOptions{
			Upsert: &upsert,
		})

		if err != nil {
			return err
		}
	}
	return nil
}

func (s *ruleStorage) FindAll(opts storage.FindOpts) ([]types.Rule, error) {
	coll := s.getRulesColl()
	var rules []rule
	query := bson.M{}
	for k, v := range opts.Metadata {
		query["metadata."+k] = v
	}
	if opts.Creator != "" {
		query["creator"] = opts.Creator
	}

	if opts.SourceTsuruApp != "" {
		query["source.tsuruapp.appname"] = opts.SourceTsuruApp
	}

	if opts.SourceTsuruJob != "" {
		query["source.tsurujob.jobname"] = opts.SourceTsuruJob
	}

	cur, err := coll.Find(context.TODO(), query, options.Find().SetSort(bson.M{"_id": 1}))
	if err != nil {
		return nil, err
	}
	err = cur.All(context.TODO(), &rules)
	if err != nil {
		return nil, err
	}
	extRules := make([]types.Rule, len(rules))
	for i := range rules {
		extRules[i] = types.Rule(rules[i])
	}
	return extRules, nil
}

func (s *ruleStorage) Delete(opts storage.DeleteOpts) error {
	coll := s.getRulesColl()
	query := bson.M{}
	if opts.ID != "" {
		query["_id"] = opts.ID
	}
	for k, v := range opts.Metadata {
		query["metadata."+k] = v
	}
	change, err := coll.UpdateMany(
		context.TODO(),
		query,
		bson.M{"$set": bson.M{"removed": true}},
	)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			err = storage.ErrRuleNotFound
		}
		return err
	}
	if change.ModifiedCount == 0 {
		return storage.ErrRuleNotFound
	}
	return nil
}
