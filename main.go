package main

import (
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"gopkg.in/yaml.v2"
)

type Policy struct {
	Policy []struct {
		Name           string   `yaml:"name"`
		Resources      []string `yaml:"resources"`
		Caseinsenstive bool     `yaml:"caseinsenstive"`
		Keys           []string `yaml:"keys"`
	} `yaml:"policy"`
}

func GetVeaverData(rawdata []byte) *Policy {
	// Get Veaver Data Struct
	var data *Policy
	data_err := yaml.Unmarshal(rawdata, &data)

	if data_err != nil {
		log.Fatal(data_err)
	}
	return data
}

func ListBucket(svc *s3.S3) (buckets []*s3.Bucket) {
	// ListBuckets
	result, err := svc.ListBuckets(&s3.ListBucketsInput{})
	if err != nil {
		log.Println("Failed to list buckets", err)
		return
	}
	return result.Buckets
}

var AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_REGION string

func VarCheck() {
	// Check if environment variables have values.
	AWS_ACCESS_KEY_ID := os.Getenv("AWS_ACCESS_KEY_ID")
	AWS_SECRET_ACCESS_KEY := os.Getenv("AWS_SECRET_ACCESS_KEY")
	AWS_REGION := os.Getenv("AWS_REGION")

	if (len(AWS_ACCESS_KEY_ID) == 0) && (len(AWS_SECRET_ACCESS_KEY) == 0) && (len(AWS_REGION) == 0) {
		fmt.Print("Provide credentials: AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_REGION\n")
		os.Exit(0)
	}
}

func GetS3TagKeys(svc *s3.S3, S3Bucket string) []string {
	// Extract Keys from TagList
	TagInput := s3.GetBucketTaggingInput{
		Bucket: &S3Bucket,
	}
	TagSet, err := svc.GetBucketTagging(&TagInput)
	if err != nil {
		fmt.Println("Error:", err)
	}
	var KeyList []string
	for value := range TagSet.TagSet {
		KeyList = append(KeyList, *TagSet.TagSet[value].Key)
	}
	return KeyList
}

func GetBucketNameList(Buckets []*s3.Bucket) []string {
	var BucketNameList []string
	for value := range Buckets {
		BucketNameList = append(BucketNameList, *Buckets[value].Name)
	}
	return BucketNameList
}

func main() {
	VarCheck()
	conf := aws.Config{Region: aws.String(AWS_REGION)}

	sess, err := session.NewSession(&conf)
	if err != nil {
		fmt.Println(err)
	}

	svc := s3.New(sess)
	Buckets := ListBucket(svc)
	BucketNameList := GetBucketNameList(Buckets)
	for _, S3Bucket := range BucketNameList {
		fmt.Println("Bucket Name:", S3Bucket)
		S3BucketList := GetS3TagKeys(svc, S3Bucket)
		fmt.Println("Bucket Tags:", S3BucketList)
	}
}
