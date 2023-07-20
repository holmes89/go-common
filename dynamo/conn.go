package dynamo

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/holmes89/go-common/query"
)

var (
	notfound    *types.ResourceNotFoundException
	ErrNotFound = errors.New("entity not found")
)

type Serializable[T any] interface {
	Serialize() (map[string]types.AttributeValue, error)
	Deserialize(map[string]types.AttributeValue) (T, error)
	DeserializeList([]map[string]types.AttributeValue) ([]T, error)
	PK() string
	SK(*string) string
}

// Conn is the connection to the Dynamodb
type Conn[T Serializable[T]] struct {
	db   *dynamodb.Client
	conf DBConf
}

type DBConf struct {
	TableName string
}

func New[T Serializable[T]](conf DBConf) *Conn[T] {
	// Load the Shared AWS Configuration (~/.aws/config)
	cfg, err := loadConfig()
	if err != nil {
		log.Println("unable to load config", err)
	}
	// Create DynamoDB client
	svc := dynamodb.NewFromConfig(cfg)

	return &Conn[T]{
		db:   svc,
		conf: conf,
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
	var t T
	params := &dynamodb.GetItemInput{
		TableName: aws.String(conn.conf.TableName),
		Key: map[string]types.AttributeValue{
			"SK": &types.AttributeValueMemberS{Value: t.SK(&id)},
			"PK": &types.AttributeValueMemberS{Value: t.PK()},
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

	rs, err = t.Deserialize(resp.Item)
	if err != nil {
		log.Println("unable to unmarshal ", err)
		return rs, errors.New("failed to scan ")
	}

	return rs, nil
}

func (conn *Conn[T]) FindByPkAndSk(ctx context.Context, pk string, sk string) (T, error) {
	var t T
	params := &dynamodb.GetItemInput{
		TableName: aws.String(conn.conf.TableName),
		Key: map[string]types.AttributeValue{
			"SK": &types.AttributeValueMemberS{Value: sk},
			"PK": &types.AttributeValueMemberS{Value: pk},
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

	rs, err = t.Deserialize(resp.Item)
	if err != nil {
		log.Println("unable to unmarshal ", err)
		return rs, errors.New("failed to scan ")
	}

	return rs, nil
}

func (conn *Conn[T]) FindAll(ctx context.Context, filter query.Opts) ([]T, error) {
	var t T
	params := &dynamodb.QueryInput{
		TableName:              aws.String(conn.conf.TableName),
		Limit:                  aws.Int32(10),
		KeyConditionExpression: aws.String("PK = :key"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":key": &types.AttributeValueMemberS{Value: t.PK()},
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

	entities, err = t.DeserializeList(resp.Items)

	if err != nil {
		log.Println("unable to unmarshal ", err)
		return entities, errors.New("unable to fetch all ")
	}
	return entities, nil
}

func (conn *Conn[T]) FindByPk(ctx context.Context, pk string, filter query.Opts) ([]T, error) {
	var t T
	params := &dynamodb.QueryInput{
		TableName:              aws.String(conn.conf.TableName),
		Limit:                  aws.Int32(10),
		KeyConditionExpression: aws.String("PK = :key"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":key": &types.AttributeValueMemberS{Value: pk},
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

	entities, err = t.DeserializeList(resp.Items)

	if err != nil {
		log.Println("unable to unmarshal ", err)
		return entities, errors.New("unable to fetch all ")
	}
	return entities, nil
}

func (conn *Conn[T]) Create(ctx context.Context, r T) (T, error) {

	rs, err := r.Serialize()
	if err != nil {
		log.Println("unable to marshal  message", err)
		return r, errors.New("failed to insert ")
	}

	fmt.Printf("table:%s\npk:%s\nsk:%s", conn.conf.TableName, r.PK(), r.SK(nil))

	params := &dynamodb.PutItemInput{
		Item:      rs,
		TableName: aws.String(conn.conf.TableName),
	}

	if _, err := conn.db.PutItem(ctx, params); err != nil {
		log.Println("unable to put message", err)
		return r, errors.New("failed to insert ")
	}
	return r, nil
}
