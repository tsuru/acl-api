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

var serviceOnce sync.Once

type serviceStorage struct {
	*mongoStorage
}

func (s *serviceStorage) getServiceColl() *mongo.Collection {
	coll := s.getCollection("acl_services")
	serviceOnce.Do(func() {
		coll.Indexes().CreateOne(context.TODO(), mongo.IndexModel{
			Keys: bson.D{
				{Key: "instancename", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		})
	})
	return coll
}

func (s *serviceStorage) Create(instance types.ServiceInstance) error {
	coll := s.getServiceColl()
	_, err := coll.InsertOne(context.TODO(), instance)
	if err != nil && mongo.IsDuplicateKeyError(err) {
		err = storage.ErrInstanceAlreadyExists
	}
	return err
}

func (s *serviceStorage) Find(instanceName string) (types.ServiceInstance, error) {
	coll := s.getServiceColl()
	var instance types.ServiceInstance
	result := coll.FindOne(context.TODO(), bson.M{"instancename": instanceName})
	err := result.Err()
	if err != nil {
		if err == mongo.ErrNoDocuments {
			err = storage.ErrInstanceNotFound
		}
		return instance, err
	}
	err = result.Decode(&instance)
	if err != nil {
		return instance, err
	}
	return instance, nil
}

func (s *serviceStorage) Delete(instanceName string) error {
	coll := s.getServiceColl()
	result, err := coll.DeleteOne(context.TODO(), bson.M{"instancename": instanceName})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return storage.ErrInstanceNotFound
	}
	return nil
}

func (s *serviceStorage) AddRule(instanceName string, r *types.ServiceRule) error {
	coll := s.getServiceColl()
	if r.RuleID == "" {
		r.RuleID = newID()
	}
	r.Created = time.Now().UTC()
	_, err := coll.UpdateOne(context.TODO(), bson.M{"instancename": instanceName}, bson.M{
		"$push": bson.M{"baserules": r},
	})
	if err != nil && err == mongo.ErrNoDocuments {
		err = storage.ErrInstanceNotFound
	}
	return err
}

func (s *serviceStorage) AddApp(instanceName string, appName string) error {
	coll := s.getServiceColl()
	_, err := coll.UpdateOne(context.TODO(), bson.M{"instancename": instanceName}, bson.M{
		"$addToSet": bson.M{"bindapps": appName},
	})
	if err != nil && err == mongo.ErrNoDocuments {
		err = storage.ErrInstanceNotFound
	}
	return err
}

func (s *serviceStorage) RemoveRule(instanceName string, ruleID string) error {
	coll := s.getServiceColl()
	_, err := coll.UpdateOne(context.TODO(), bson.M{"instancename": instanceName}, bson.M{
		"$pull": bson.M{"baserules": bson.M{"rule.ruleid": ruleID}},
	})
	if err != nil && err == mongo.ErrNoDocuments {
		err = storage.ErrInstanceNotFound
	}
	return err
}

func (s *serviceStorage) RemoveApp(instanceName string, appName string) error {
	coll := s.getServiceColl()
	_, err := coll.UpdateOne(context.TODO(), bson.M{"instancename": instanceName}, bson.M{
		"$pull": bson.M{"bindapps": appName},
	})
	if err != nil && err == mongo.ErrNoDocuments {
		err = storage.ErrInstanceNotFound
	}
	return err
}

func (s *serviceStorage) List() ([]types.ServiceInstance, error) {
	coll := s.getServiceColl()
	var ret []types.ServiceInstance
	cur, err := coll.Find(context.TODO(), bson.M{})
	if err != nil {
		return nil, err
	}
	err = cur.All(context.TODO(), &ret)
	return ret, err
}
