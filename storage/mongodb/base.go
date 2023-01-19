// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mongodb

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
	"github.com/tsuru/acl-api/storage"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/mgocompat"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
)

// once global is only reset in tests
var once sync.Once

func init() {
	var client *mongo.Client
	var database string

	createConn := func() (stor *mongoStorage, err error) {
		once.Do(func() {
			var addr string

			// compability with https://github.com/globocom/database-as-a-service
			addr = viper.GetString("dbaas_mongodb_endpoint")

			if addr == "" {
				addr = viper.GetString("storage")
			}

			var cs connstring.ConnString
			cs, err = connstring.ParseAndValidate(addr)
			if err != nil {
				return
			}
			database = cs.Database

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			opts := options.Client().ApplyURI(addr).
				SetSocketTimeout(1 * time.Minute).
				SetServerSelectionTimeout(20 * time.Second).
				SetConnectTimeout(30 * time.Second).
				SetRegistry(mgocompat.Registry)

			client, err = mongo.Connect(ctx, opts)
			if err != nil {
				return
			}
			err = client.Ping(ctx, readpref.Primary())
			if err != nil {
				return
			}
		})
		if err != nil {
			once = sync.Once{}
			return nil, err
		}
		return &mongoStorage{client: client, database: database}, nil
	}

	storage.GetRuleStorage = func() (storage.RuleStorage, error) {
		stor, err := createConn()
		if err != nil {
			return nil, err
		}
		return &ruleStorage{stor}, nil
	}

	storage.GetServiceStorage = func() (storage.ServiceStorage, error) {
		stor, err := createConn()
		if err != nil {
			return nil, err
		}
		return &serviceStorage{stor}, nil
	}

	storage.GetSyncStorage = func() (storage.SyncStorage, error) {
		stor, err := createConn()
		if err != nil {
			return nil, err
		}
		return &syncStorage{stor}, nil
	}

	storage.GetACLAPIStorage = func() (storage.ACLAPIStorage, error) {
		stor, err := createConn()
		if err != nil {
			return nil, err
		}
		return &aclapiStorage{stor}, nil
	}
}

func newID() string {
	return primitive.NewObjectID().Hex()
}

type mongoStorage struct {
	client   *mongo.Client
	database string
}

func (s *mongoStorage) getCollection(name string) *mongo.Collection {
	return s.client.Database(s.database).Collection(name)
}

// ClearAll will remove all collections and must only be used in tests
func (s *mongoStorage) ClearAll() {
	collections, _ := s.client.Database(s.database).ListCollectionNames(context.TODO(), bson.M{})
	for _, c := range collections {
		if strings.HasPrefix(c, "acl_") {
			coll := s.getCollection(c)
			coll.DeleteMany(context.Background(), bson.M{})
		}
	}
}
