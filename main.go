package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/lambda"
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
	TagCheck := true
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
			} else if len(PolicyTagList)-1 == index && TagCheck == true {
				fmt.Println("TagCheck Success.")
				fmt.Println("TagCheck Passed.")
				TaggedS3Bucket = append(TaggedS3Bucket, S3Bucket)
			}
		}
	}
	return TaggedS3Bucket, UnTaggedS3Bucket
}

func S3Init(PolicyObject *Policy, sess *session.Session) {
	svc := s3.New(sess)
	Buckets := ListBucket(svc)
	BucketNameList := GetBucketNameList(Buckets)
	Tagged, UnTagged := S3TagFinder(svc, BucketNameList, GetPolicyKeys(PolicyObject))

	fmt.Println("Final Tagged:", Tagged, "Final UnTagged:", UnTagged)
}

func check(e error) {

	if e != nil {
		panic(e)
	}
}

func GetEc2TagKeys(ec2List *ec2.Instance) []string {
	var KeyList []string

	for index := range ec2List.Tags {
		KeyList = append(KeyList, *ec2List.Tags[index].Key)
	}
	return KeyList
}

func Ec2TagFinder(Ec2List []*ec2.Instance, PolicyTagList []string) ([]string, []string) {
	TagCheck := true
	var TaggedEC2Instances, UnTaggedEC2Instances []string
	for _, EC2Instance := range Ec2List {
		fmt.Println("\n\n")
		fmt.Println("Instance Name: ", EC2Instance.InstanceId)
		Ec2TagList := GetEc2TagKeys(EC2Instance)
		for index, PolicyTag := range PolicyTagList {
			fmt.Println("Policy Tag: ", index, " ", PolicyTag)
			TagCheck = contains(Ec2TagList, PolicyTag)
			if TagCheck == false {
				fmt.Println("TagCheck Failed.")
				UnTaggedEC2Instances = append(UnTaggedEC2Instances, *EC2Instance.InstanceId)
				break
			} else if len(PolicyTagList)-1 == index && TagCheck == true {
				fmt.Println("TagCheck Success.")
				fmt.Println("TagCheck Passed.")
				TaggedEC2Instances = append(TaggedEC2Instances, *EC2Instance.InstanceId)
			}
		}
	}
	return TaggedEC2Instances, UnTaggedEC2Instances
}

func EC2Init(PolicyObject *Policy, sess *session.Session) {
	svc := ec2.New(sess)
	params := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			&ec2.Filter{
				Name: aws.String("instance-state-name"),
				Values: []*string{
					aws.String("running"),
					aws.String("pending"),
				},
			},
		},
	}

	// TODO: Actually care if we can't connect to a host
	resp, err := svc.DescribeInstances(params)
	check(err)

	var InstancesList []*ec2.Instance
	for idx, _ := range resp.Reservations {
		InstancesList = append(InstancesList, resp.Reservations[idx].Instances...)
		for _, inst := range resp.Reservations[idx].Instances {
			fmt.Println("    - Instance ID: ", *inst.InstanceId)
		}
	}
	fmt.Println(len(InstancesList))
	fmt.Println("Instances List:", InstancesList)
	Tagged, UnTagged := Ec2TagFinder(InstancesList, GetPolicyKeys(PolicyObject))
	fmt.Println("Final Tagged:", Tagged)
	fmt.Println("Final UnTagged:", UnTagged)
}

func GetElbTagKeys(tagList []*elb.Tag) []string {
	var KeyList []string

	for index := range tagList {
		KeyList = append(KeyList, *tagList[index].Key)
	}
	return KeyList
}

func ElbTagFinder(svc *elb.ELB, ElbList []*elb.LoadBalancerDescription, PolicyTagList []string) ([]string, []string) {
	TagCheck := true
	var TaggedElb, UnTaggedElb, ElbTagList []string
	var LoadBalancerNamesList []*string
	for _, Elb := range ElbList {
		fmt.Println("\n\n")
		fmt.Println("ELB Name: ", *Elb.LoadBalancerName)
		LoadBalancerNamesList = append(LoadBalancerNamesList, Elb.LoadBalancerName)
		TagInputs := elb.DescribeTagsInput{
			LoadBalancerNames: LoadBalancerNamesList,
		}
		ELB_Tags, err := svc.DescribeTags(&TagInputs)
		check(err)
		for _, val := range ELB_Tags.TagDescriptions {
			ElbTagList = GetElbTagKeys(val.Tags)
			fmt.Println(ElbTagList)
		}
		for index, PolicyTag := range PolicyTagList {
			fmt.Println("Policy Tag: ", index, " ", PolicyTag)
			TagCheck = contains(ElbTagList, PolicyTag)
			if TagCheck == false {
				fmt.Println("TagCheck Failed.")
				UnTaggedElb = append(TaggedElb, *Elb.LoadBalancerName)
				break
			} else if len(PolicyTagList)-1 == index && TagCheck == true {
				fmt.Println("TagCheck Success.")
				TaggedElb = append(UnTaggedElb, *Elb.LoadBalancerName)
			}
		}
		LoadBalancerNamesList = nil
	}
	return TaggedElb, UnTaggedElb
}

func ELBInit(PolicyObject *Policy, sess *session.Session) {

	svc := elb.New(sess)
	input := &elb.DescribeLoadBalancersInput{}
	result, _ := svc.DescribeLoadBalancers(input)
	Tagged, UnTagged := ElbTagFinder(svc, result.LoadBalancerDescriptions, GetPolicyKeys(PolicyObject))

	fmt.Println("\n\n\n\nFinal Tagged:", Tagged)
	fmt.Println("Final UnTagged:", UnTagged)
}

func GetElbTargetGroupTagKeys(tagList []*elbv2.Tag) []string {
	var KeyList []string

	for index := range tagList {
		KeyList = append(KeyList, *tagList[index].Key)
	}
	return KeyList
}

func ElbTargetGroupFinder(svc *elbv2.ELBV2, ElbTargetGroupList []*elbv2.TargetGroup, PolicyTagList []string) ([]string, []string) {
	TagCheck := true
	var TaggedElbTargetGroup, UnTaggedElbTargetGroup, ElbTargetGroupTagList []string
	var ElbTargetGroupArnList []*string
	for _, ElbTargetGroup := range ElbTargetGroupList {
		fmt.Println("Name: ", *ElbTargetGroup.TargetGroupName)
		fmt.Println("Arn: ", *ElbTargetGroup.TargetGroupArn)
		ElbTargetGroupArnList = append(ElbTargetGroupArnList, *&ElbTargetGroup.TargetGroupArn)
		TagInputs := elbv2.DescribeTagsInput{
			ResourceArns: ElbTargetGroupArnList,
		}
		ElbTargetGroup_Tags, err := svc.DescribeTags(&TagInputs)
		check(err)
		for _, val := range ElbTargetGroup_Tags.TagDescriptions {
			ElbTargetGroupTagList = GetElbTargetGroupTagKeys(val.Tags)
			fmt.Println(ElbTargetGroupTagList)
		}
		for index, PolicyTag := range PolicyTagList {
			fmt.Println("Policy Tag: ", index, " ", PolicyTag)
			TagCheck = contains(ElbTargetGroupTagList, PolicyTag)
			if TagCheck == false {
				fmt.Println("TagCheck Failed.")
				UnTaggedElbTargetGroup = append(UnTaggedElbTargetGroup, *ElbTargetGroup.TargetGroupName)
				break
			} else if len(PolicyTagList)-1 == index && TagCheck == true {
				fmt.Println("TagCheck Success.")
				TaggedElbTargetGroup = append(TaggedElbTargetGroup, *ElbTargetGroup.TargetGroupName)
			}
		}
	}

	return TaggedElbTargetGroup, UnTaggedElbTargetGroup
}

func ElbTargetGroupInit(PolicyObject *Policy, sess *session.Session) {
	svc := elbv2.New(sess)
	input := &elbv2.DescribeTargetGroupsInput{}
	result, _ := svc.DescribeTargetGroups(input)
	Tagged, UnTagged := ElbTargetGroupFinder(svc, result.TargetGroups, GetPolicyKeys(PolicyObject))
	fmt.Println("\n\n\n\nFinal Tagged:", Tagged)
	fmt.Println("Final UnTagged:", UnTagged)
}

func LambdaFinder(svc *lambda.Lambda, LambdaList []*lambda.FunctionConfiguration, PolicyTagList []string) ([]string, []string) {
	TagCheck := true
	fmt.Println("XXXX")
	var TaggedLambda, UnTaggedLambda []string
	var LambdaTagList []string
	// var LambdaArnList []*string
	for _, Lambda := range LambdaList {
		fmt.Println("\n\n\nName: ", *Lambda.FunctionName)
		fmt.Println("Arn: ", *Lambda.FunctionArn)
		// LambdaArnList = append(LambdaArnList)
		TagInputs := lambda.ListTagsInput{
			Resource: *&Lambda.FunctionArn,
		}
		LambdaTags, err := svc.ListTags(&TagInputs)
		fmt.Println(LambdaTags.Tags)
		check(err)
		for index, _ := range LambdaTags.Tags {
			LambdaTagList = append(LambdaTagList, index)
		}
		fmt.Println("-->", LambdaTagList)
		for index, PolicyTag := range PolicyTagList {
			fmt.Println("Policy Tag: ", index, " ", PolicyTag)
			TagCheck = contains(LambdaTagList, PolicyTag)
			if TagCheck == false {
				fmt.Println("TagCheck Failed.")
				UnTaggedLambda = append(UnTaggedLambda, *Lambda.FunctionName)
				break
			} else if len(PolicyTagList)-1 == index && TagCheck == true {
				fmt.Println("TagCheck Success.")
				TaggedLambda = append(TaggedLambda, *Lambda.FunctionName)
			}
		}
		LambdaTagList = nil
	}
	return TaggedLambda, UnTaggedLambda
}

func LambdaInit(PolicyObject *Policy, sess *session.Session) {
	svc := lambda.New(sess)
	fmt.Println(svc)
	result, err := svc.ListFunctions(nil)
	if err != nil {
		fmt.Println("Cannot list functions")
		os.Exit(0)
	}
	// for _, f := range result.Functions {
	// 	fmt.Println("Name:        " + aws.StringValue(f.FunctionName))
	// }
	Tagged, UnTagged := LambdaFinder(svc, result.Functions, GetPolicyKeys(PolicyObject))
	fmt.Println("\n\n\n\nFinal Tagged:", Tagged)
	fmt.Println("Final UnTagged:", UnTagged)
}

func main() {
	VarCheck()

	PolicyObject := GetPolicyData("policy.yaml")
	conf := aws.Config{Region: aws.String(AWS_REGION)}

	sess, err := session.NewSession(&conf)
	if err != nil {
		fmt.Println(err)
	}
	// S3Init(PolicyObject,sess)
	// EC2Init(PolicyObject, sess)
	// ELBInit(PolicyObject, sess)
	// ElbTargetGroupInit(PolicyObject, sess)
	LambdaInit(PolicyObject, sess)
}
