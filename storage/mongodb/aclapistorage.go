// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mongodb

import (
	"context"
	"sync"

	"github.com/tsuru/acl-api/storage"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	_ storage.ACLAPIStorage = &aclapiStorage{}

	aclapiOnce sync.Once
)

type aclapiStorage struct {
	*mongoStorage
}

func (s *aclapiStorage) getACLAPIColl() *mongo.Collection {
	coll := s.getCollection("acl_aclapi")
	aclapiOnce.Do(func() {
		coll.Indexes().CreateOne(context.TODO(), mongo.IndexModel{
			Keys: bson.D{
				{Key: "ruleid", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		})
	})
	return coll
}

func (s *aclapiStorage) Find(ruleID string) (storage.ACLAPISyncedRule, error) {
	coll := s.getACLAPIColl()
	var r storage.ACLAPISyncedRule
	result := coll.FindOne(context.TODO(), bson.M{"ruleid": ruleID})
	err := result.Err()
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return r, storage.ErrACLAPISyncedRuleNotFound
		}
		return r, err
	}
	err = result.Decode(&r)
	return r, err
}

func (s *aclapiStorage) Add(ruleID string, aclIDs []storage.ACLIdPair) error {
	coll := s.getACLAPIColl()
	_, err := coll.UpdateOne(context.TODO(),
		bson.M{"ruleid": ruleID},
		bson.M{
			"$addToSet": bson.M{
				"aclids": bson.M{"$each": aclIDs},
			},
		},
		options.Update().SetUpsert(true),
	)
	return err
}

func (s *aclapiStorage) Remove(ruleID string, aclIDs []storage.ACLIdPair) error {
	coll := s.getACLAPIColl()
	_, err := coll.UpdateOne(context.TODO(),
		bson.M{"ruleid": ruleID},
		bson.M{
			"$pullAll": bson.M{
				"aclids": aclIDs,
			},
		},
	)
	if err == mongo.ErrNoDocuments {
		err = storage.ErrACLAPISyncedRuleNotFound
	}
	return err
}
