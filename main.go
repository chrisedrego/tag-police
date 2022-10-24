package main

import (
	"fmt"
	"io/ioutil"
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

func GetPolicyData(filePath string) *Policy {
	// Read Policy Config file.
	yamlFile, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Printf("yamlFile.Get err   #%v ", err)
	}

	var data *Policy
	data_err := yaml.Unmarshal(yamlFile, &data)

	if data_err != nil {
		log.Fatal(data_err)
	}
	return data
}

func GetPolicyKeys(PolicyObject *Policy) []string {
	return PolicyObject.Policy[0].Keys
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

func contains(elems []string, v string) bool {
	ContainsFlag := false
	for _, s := range elems {
		if v == s {
			fmt.Println("Comparing: ", v, " : ", s)
			ContainsFlag = true
		}
	}
	return ContainsFlag
}

func S3TagFinder(svc *s3.S3, BucketNameList, PolicyTagList []string) ([]string, []string) {
	var TagCheck bool
	var TaggedS3Bucket, UnTaggedS3Bucket []string
	for _, S3Bucket := range BucketNameList {
		fmt.Println("\n\n")
		fmt.Println("Bucket Name: ", S3Bucket)
		S3BucketTagList := GetS3TagKeys(svc, S3Bucket)
		for index, PolicyTag := range PolicyTagList {
			fmt.Println("Policy Tag: ", index, " ", PolicyTag)
			TagCheck = contains(S3BucketTagList, PolicyTag)
			if TagCheck == false {
				fmt.Println("TagCheck Failed.")
				UnTaggedS3Bucket = append(UnTaggedS3Bucket, S3Bucket)
				break
			} else {
				fmt.Println("TagCheck Success.")
			}
			if len(PolicyTagList) == index && TagCheck == true {
				fmt.Println("TagCheck Passed.")
				TaggedS3Bucket = append(TaggedS3Bucket, S3Bucket)
			}
		}
	}
	return TaggedS3Bucket, UnTaggedS3Bucket
}

func main() {
	VarCheck()

	PolicyObject := GetPolicyData("policy.yaml")
	conf := aws.Config{Region: aws.String(AWS_REGION)}

	sess, err := session.NewSession(&conf)
	if err != nil {
		fmt.Println(err)
	}

	svc := s3.New(sess)
	Buckets := ListBucket(svc)
	BucketNameList := GetBucketNameList(Buckets)
	Tagged, UnTagged := S3TagFinder(svc, BucketNameList, GetPolicyKeys(PolicyObject))

	fmt.Println("Final Tagged:", Tagged, "Final UnTagged:", UnTagged)
}
