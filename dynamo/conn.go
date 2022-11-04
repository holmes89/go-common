package database

import (
	"context"
	"errors"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/holmes89/go-common/query"
)

var (
	notfound    *types.ResourceNotFoundException
	ErrNotFound = errors.New("entity not found")
)

type typeable interface {
	Type() string
}

// Conn is the connection to the Dynamodb
type Conn[T typeable] struct {
	db        *dynamodb.Client
	tableName string
}

func New[T typeable](tableName string) *Conn[T] {
	// Load the Shared AWS Configuration (~/.aws/config)
	cfg, err := loadConfig()
	if err != nil {
		log.Println("unable to load config", err)
	}
	// Create DynamoDB client
	svc := dynamodb.NewFromConfig(cfg)

	return &Conn[T]{
		db: svc,
	}
}

func loadConfig() (aws.Config, error) {
	if conn := os.Getenv("DYNAMODB_ENDPOINT"); conn != "" {
		log.Println("using local database connection")
		return config.LoadDefaultConfig(context.TODO(),
			config.WithRegion(("us-east-1")),
			config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{URL: "http://dynamo:8000", SigningRegion: "us-east-1"}, nil
			})))
	}
	return config.LoadDefaultConfig(context.Background())
}

func (conn *Conn[T]) FindByID(ctx context.Context, id string) (T, error) {
	params := &dynamodb.GetItemInput{
		TableName: aws.String(conn.tableName),
		Key: map[string]types.AttributeValue{
			"SK": &types.AttributeValueMemberS{Value: id},
			"ID": &types.AttributeValueMemberS{Value: ""},
		},
	}

	var rs T
	resp, err := conn.db.GetItem(ctx, params)
	if err != nil {
		if errors.As(err, &notfound) {
			log.Println("no resources found")
			return rs, nil
		}
		log.Println("unable to find ", err)
		return rs, errors.New("unable to fetch ")
	}

	if err := attributevalue.UnmarshalMap(resp.Item, &rs); err != nil {
		log.Println("unable to unmarshal ", err)
		return rs, errors.New("failed to scan ")
	}

	return rs, nil
}

func (conn *Conn[T]) FindAll(ctx context.Context, filter query.Opts) ([]T, error) {
	var t T
	params := &dynamodb.QueryInput{
		TableName:              aws.String(conn.tableName),
		Limit:                  aws.Int32(10),
		KeyConditionExpression: aws.String("ID = :key"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":key": &types.AttributeValueMemberS{Value: t.Type()},
		},
		ScanIndexForward: aws.Bool(false),
	}

	entities := make([]T, 0)
	resp, err := conn.db.Query(ctx, params)
	if err != nil {
		if errors.As(err, &notfound) {
			log.Println("no resources found")
			return entities, nil
		}
		log.Println("unable to fetch ", err)
		return entities, errors.New("unable to fetch all ")
	}
	if err := attributevalue.UnmarshalListOfMaps(resp.Items, &entities); err != nil {
		log.Println("unable to unmarshal ", err)
		return entities, errors.New("unable to fetch all ")
	}
	return entities, nil
}

type entity[T typeable] struct {
	Entity     T
	EntityType string `json:"-" dynamodbav:"ID"`
}

func (conn *Conn[T]) Create(ctx context.Context, r T) (T, error) {
	e := entity[T]{
		Entity:     r,
		EntityType: r.Type(),
	}
	rs, err := attributevalue.MarshalMap(e)
	if err != nil {
		log.Println("unable to marshal  message", err)
		return r, errors.New("failed to insert ")
	}

	params := &dynamodb.PutItemInput{
		Item:      rs,
		TableName: aws.String(conn.tableName),
	}
	if _, err := conn.db.PutItem(ctx, params); err != nil {
		log.Println("unable to put message", err)
		return r, errors.New("failed to insert ")
	}
	return r, nil
}
