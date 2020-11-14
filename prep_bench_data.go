package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"vector-search-go/db"

	"gonum.org/v1/hdf5"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var (
	dbLocation          = os.Getenv("MONGO_ADDR")
	dbName              = os.Getenv("DB_NAME")
	batchSize, _        = strconv.Atoi(os.Getenv("BATCH_SIZE"))
	trainCollectionName = os.Getenv("TRAIN_COLLECTION_NAME")
	testCollectionName  = os.Getenv("TEST_COLLECTION_NAME")
)

func main() {
	client, err := mongo.NewClient(options.Client().ApplyURI(dbLocation))
	if err != nil {
		log.Fatal(err)
	}

	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	err = client.Connect(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect(ctx)

	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		log.Fatal(err)
	}

	database := client.Database(dbName)
	vectorsTrainCollection := database.Collection(trainCollectionName)
	vectorsTestCollection := database.Collection(testCollectionName)

	f, err := hdf5.OpenFile("./db/deep-image-96-angular.hdf5", hdf5.F_ACC_RDWR)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	{
		featuresTest, err := db.GetVectorsFromHDF5(f, "test")
		if err != nil {
			log.Fatal(err)
		}
		neighbors, err := db.GetNeighborsFromHDF5(f, "neighbors")
		if err != nil {
			log.Fatal(err)
		}

		err = db.LoadDatasetMongoDb(vectorsTestCollection, featuresTest, neighbors, batchSize)
		if err != nil {
			log.Fatal(err)
		}
		log.Println("Test data has been saved to mongo!")
	}

	{
		featuresTrain, err := db.GetVectorsFromHDF5(f, "train")
		if err != nil {
			log.Fatal(err)
		}
		err = db.LoadDatasetMongoDb(vectorsTrainCollection, featuresTrain, []db.NeighborsIds{}, batchSize)
		if err != nil {
			log.Fatal(err)
		}
		log.Println("Train data has been saved to mongo!")
	}
}
