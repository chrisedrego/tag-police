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
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/workspaces"
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

func RdsTagFinder(Ec2List []*ec2.Instance, PolicyTagList []string) ([]string, []string) {
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

func RDSInit(PolicyObject *Policy, sess *session.Session) {
	svc := rds.New(sess)
	input := rds.DescribeDBInstancesInput{}
	result, err := svc.DescribeDBInstances(&input)
	if err != nil {
		fmt.Println("Cannot list functions")
		os.Exit(0)
	}
	fmt.Println(result)
	for _, f := range result.DBInstances {
		fmt.Println(f)
	}
	// Tagged, UnTagged := LambdaFinder(svc, result.Functions, GetPolicyKeys(PolicyObject))
	// fmt.Println("\n\n\n\nFinal Tagged:", Tagged)
	// fmt.Println("Final UnTagged:", UnTagged)
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

func Route53Finder(svc *route53.Route53, Route53List []*route53.HostedZone, PolicyTagList []string) ([]string, []string) {
	TagCheck := true
	fmt.Println("Route53")
	var Route53TagList []string
	var TaggedLambda, UnTaggedLambda []string
	// var ResourceIds []*string
	ResourceType := "hostedzone"
	for _, Route53 := range Route53List {
		fmt.Println("\n\n\nName: ", *Route53.Name)
		fmt.Println("\n\n\nHostedID: ", *Route53.Id)

		input := route53.ListTagsForResourceInput{
			ResourceId:   Route53.Id,
			ResourceType: &ResourceType,
		}
		tagObject, _ := svc.ListTagsForResource(&input)
		for index := range tagObject.ResourceTagSet.Tags {
			Route53TagList = append(Route53TagList, *tagObject.ResourceTagSet.Tags[index].Key)
		}
		fmt.Println("-->", Route53TagList)
		for index, PolicyTag := range PolicyTagList {
			fmt.Println("Policy Tag: ", index, " ", PolicyTag)
			TagCheck = contains(Route53TagList, PolicyTag)
			if TagCheck == false {
				fmt.Println("TagCheck Failed.")
				UnTaggedLambda = append(UnTaggedLambda, *Route53.Name)
				break
			} else if len(PolicyTagList)-1 == index && TagCheck == true {
				fmt.Println("TagCheck Success.")
				TaggedLambda = append(TaggedLambda, *Route53.Name)
			}
		}
	}
	return TaggedLambda, UnTaggedLambda
}

func Route53Init(PolicyObject *Policy, sess *session.Session) {
	svc := route53.New(sess)
	input := route53.ListHostedZonesInput{}
	result, err := svc.ListHostedZones(&input)
	if err != nil {
		fmt.Println("Cannot list Route53")
		os.Exit(0)
	}

	for _, f := range result.HostedZones {
		fmt.Println(*f)
	}
	Tagged, UnTagged := Route53Finder(svc, result.HostedZones, GetPolicyKeys(PolicyObject))
	fmt.Println("\n\n\n\nFinal Tagged:", Tagged)
	fmt.Println("Final UnTagged:", UnTagged)
}

func SQSFinder(svc *sqs.SQS, QueueUrls []*string, PolicyTagList []string) ([]string, []string) {
	TagCheck := true
	fmt.Println("SQS")
	var SQSTagList []string
	var TaggedSQS, UnTaggedSQS []string
	for _, URL := range QueueUrls {

		// input := route53.ListTagsForResourceInput{
		// 	ResourceId:   Route53.Id,
		// 	ResourceType: &ResourceType,
		// }
		input := sqs.ListQueueTagsInput{
			QueueUrl: URL,
		}
		tagObject, _ := svc.ListQueueTags(&input)
		for Tag := range tagObject.Tags {
			SQSTagList = append(SQSTagList, Tag)
		}
		fmt.Println("-->", SQSTagList)
		for index, PolicyTag := range PolicyTagList {
			fmt.Println("Policy Tag: ", index, " ", PolicyTag)
			TagCheck = contains(SQSTagList, PolicyTag)
			if TagCheck == false {
				fmt.Println("TagCheck Failed.")
				UnTaggedSQS = append(UnTaggedSQS, *URL)
				break
			} else if len(PolicyTagList)-1 == index && TagCheck == true {
				fmt.Println("TagCheck Success.")
				TaggedSQS = append(TaggedSQS, *URL)
			}
		}
	}
	return TaggedSQS, UnTaggedSQS
}

func WorkspacesFinder(svc *workspaces.WorkSpaces, Workspaces []*workspaces.Workspace, PolicyTagList []string) ([]string, []string) {
	TagCheck := true
	fmt.Println("Workspaces:")
	var WorkspacesTagList []string
	var TaggedWorkspaces, UnTaggedWorkspaces []string
	for _, Workspace := range Workspaces {

		input := workspaces.DescribeTagsInput{
			ResourceId: *&Workspace.WorkspaceId,
		}
		tagObject, _ := svc.DescribeTags(&input)
		for index, _ := range tagObject.TagList {
			WorkspacesTagList = append(WorkspacesTagList, *tagObject.TagList[index].Key)
		}
		fmt.Println("-->", WorkspacesTagList)
		for index, PolicyTag := range PolicyTagList {
			fmt.Println("Policy Tag: ", index, " ", PolicyTag)
			TagCheck = contains(WorkspacesTagList, PolicyTag)
			if TagCheck == false {
				fmt.Println("TagCheck Failed.")
				UnTaggedWorkspaces = append(UnTaggedWorkspaces, *Workspace.WorkspaceId)
				break
			} else if len(PolicyTagList)-1 == index && TagCheck == true {
				fmt.Println("TagCheck Success.")
				TaggedWorkspaces = append(TaggedWorkspaces, *Workspace.WorkspaceId)
			}
		}
	}
	return TaggedWorkspaces, UnTaggedWorkspaces
}

func SQSInit(PolicyObject *Policy, sess *session.Session) {
	svc := sqs.New(sess)
	input := sqs.ListQueuesInput{}
	result, err := svc.ListQueues(&input)
	if err != nil {
		fmt.Println("Cannot list Route53")
		os.Exit(0)
	}

	for _, f := range result.QueueUrls {
		fmt.Println(*f)
	}

	Tagged, UnTagged := SQSFinder(svc, result.QueueUrls, GetPolicyKeys(PolicyObject))
	fmt.Println("\n\n\n\nFinal Tagged:", Tagged)
	fmt.Println("Final UnTagged:", UnTagged)
}

func WorkspacesInit(PolicyObject *Policy, sess *session.Session) {
	svc := workspaces.New(sess)
	input := workspaces.DescribeWorkspacesInput{}
	result, err := svc.DescribeWorkspaces(&input)
	if err != nil {
		fmt.Println("Cannot list Route53")
		os.Exit(0)
	}

	for _, workspace := range result.Workspaces {
		fmt.Println(*workspace)
	}

	Tagged, UnTagged := WorkspacesFinder(svc, result.Workspaces, GetPolicyKeys(PolicyObject))
	fmt.Println("\n\n\n\nFinal Tagged:", Tagged)
	fmt.Println("Final UnTagged:", UnTagged)
}

func ElasticIpFinder(svc *ec2.EC2, ElasticIps []*ec2.Address, PolicyTagList []string) ([]string, []string) {
	TagCheck := true
	fmt.Println("Workspaces:")
	var ElasticIpTagList []string
	var TaggedElasticIp, UnTaggedElasticIp []string
	for _, eip := range ElasticIps {
		for index, _ := range eip.Tags {
			ElasticIpTagList = append(ElasticIpTagList, *eip.Tags[index].Key)
		}
		fmt.Println("-->", ElasticIpTagList)
		for index, PolicyTag := range PolicyTagList {
			fmt.Println("Policy Tag: ", index, " ", PolicyTag)
			TagCheck = contains(ElasticIpTagList, PolicyTag)
			if TagCheck == false {
				fmt.Println("TagCheck Failed.")
				UnTaggedElasticIp = append(UnTaggedElasticIp, *eip.AllocationId)
				break
			} else if len(PolicyTagList)-1 == index && TagCheck == true {
				fmt.Println("TagCheck Success.")
				TaggedElasticIp = append(TaggedElasticIp, *eip.AllocationId)
			}
		}
		ElasticIpTagList = nil
	}
	return TaggedElasticIp, UnTaggedElasticIp
}

func ElasticIpInit(PolicyObject *Policy, sess *session.Session) {
	svc := ec2.New(sess)
	result, err := svc.DescribeAddresses(&ec2.DescribeAddressesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("domain"),
				Values: aws.StringSlice([]string{"vpc"}),
			},
		},
	})
	if err != nil {
		fmt.Print("Unable to elastic IP address, %v", err)
	}

	if len(result.Addresses) == 0 {
		fmt.Printf("No elastic IPs for %s region\n", *svc.Config.Region)
	} else {
		Tagged, UnTagged := ElasticIpFinder(svc, result.Addresses, GetPolicyKeys(PolicyObject))
		fmt.Println("\n\n\n\nFinal Tagged:", Tagged)
		fmt.Println("Final UnTagged:", UnTagged)
	}
}

func fmtAddress(addr *ec2.Address) string {
	out := fmt.Sprintf("IP: %s,  allocation id: %s",
		aws.StringValue(addr.PublicIp), aws.StringValue(addr.AllocationId))
	if addr.InstanceId != nil {
		out += fmt.Sprintf(", instance-id: %s", *addr.InstanceId)
	}
	return out
}

func exitErrorf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
	os.Exit(1)
}

func AmiFinder(svc *ec2.EC2, AmiList []*ec2.Image, PolicyTagList []string) ([]string, []string) {
	TagCheck := true
	fmt.Println("Workspaces:")
	var AmiTagList []string
	var TaggedAmi, UnTaggedAmi []string
	for _, ami := range AmiList {
		for index, _ := range ami.Tags {
			AmiTagList = append(AmiTagList, *ami.Tags[index].Key)
		}
		fmt.Println("-->", AmiTagList)
		for index, PolicyTag := range PolicyTagList {
			fmt.Println("Policy Tag: ", index, " ", PolicyTag)
			TagCheck = contains(AmiTagList, PolicyTag)
			if TagCheck == false {
				fmt.Println("TagCheck Failed.")
				UnTaggedAmi = append(UnTaggedAmi, *ami.Name)
				break
			} else if len(PolicyTagList)-1 == index && TagCheck == true {
				fmt.Println("TagCheck Success.")
				TaggedAmi = append(TaggedAmi, *ami.Name)
			}
		}
		// ElasticIpTagList = nil
	}
	return TaggedAmi, UnTaggedAmi
}

func AmiInit(PolicyObject *Policy, sess *session.Session) {
	svc := ec2.New(sess)
	Owner := "self"
	var Owners []*string
	Owners = append(Owners, &Owner)
	input := ec2.DescribeImagesInput{
		Owners: Owners,
	}
	result, err := svc.DescribeImages(&input)
	if err != nil {
		fmt.Print("Unable to load AMI %v", err)
	}

	if len(result.Images) == 0 {
		fmt.Printf("No elastic IPs for %s region\n", *svc.Config.Region)
	} else {
		Tagged, UnTagged := AmiFinder(svc, result.Images, GetPolicyKeys(PolicyObject))
		fmt.Println("\n\n\n\nFinal Tagged:", Tagged)
		fmt.Println("Final UnTagged:", UnTagged)
	}
}

func InternetGatewayFinder(svc *ec2.EC2, InternetGatewayList []*ec2.InternetGateway, PolicyTagList []string) ([]string, []string) {
	TagCheck := true
	fmt.Println("Internet Gateway:")
	var InternetGatewayTagList []string
	var TaggedInternetGateway, UnTaggedInternetGateway []string
	for _, InternetGateway := range InternetGatewayList {
		for index, _ := range InternetGateway.Tags {
			InternetGatewayTagList = append(InternetGatewayTagList, *InternetGateway.Tags[index].Key)
		}
		fmt.Println("-->", InternetGatewayTagList)
		for index, PolicyTag := range PolicyTagList {
			fmt.Println("Policy Tag: ", index, " ", PolicyTag)
			TagCheck = contains(InternetGatewayTagList, PolicyTag)
			if TagCheck == false {
				fmt.Println("TagCheck Failed.")
				UnTaggedInternetGateway = append(UnTaggedInternetGateway, *InternetGateway.InternetGatewayId)
				break
			} else if len(PolicyTagList)-1 == index && TagCheck == true {
				fmt.Println("TagCheck Success.")
				TaggedInternetGateway = append(TaggedInternetGateway, *InternetGateway.InternetGatewayId)
			}
		}
		InternetGatewayTagList = nil
	}
	return TaggedInternetGateway, UnTaggedInternetGateway
}

func InternetGatewayInit(PolicyObject *Policy, sess *session.Session) {
	svc := ec2.New(sess)

	input := ec2.DescribeInternetGatewaysInput{}
	result, err := svc.DescribeInternetGateways(&input)
	if err != nil {
		fmt.Print("Unable to load InternetGateway %v", err)
	}

	if len(result.InternetGateways) == 0 {
		fmt.Printf("No InternetGateway for %s region\n", *svc.Config.Region)
	} else {
		fmt.Println(result.InternetGateways)
		Tagged, UnTagged := InternetGatewayFinder(svc, result.InternetGateways, GetPolicyKeys(PolicyObject))
		fmt.Println("\n\n\n\nFinal Tagged:", Tagged)
		fmt.Println("Final UnTagged:", UnTagged)
	}
}

func NatGatewayFinder(svc *ec2.EC2, NatGatewayList []*ec2.NatGateway, PolicyTagList []string) ([]string, []string) {
	TagCheck := true
	fmt.Println("NatGateway:")
	var NatGatewayTagList []string
	var TaggedNatGateway, UnTaggedNatGateway []string
	for _, NatGateway := range NatGatewayList {
		for index, _ := range NatGateway.Tags {
			NatGatewayTagList = append(NatGatewayTagList, *NatGateway.Tags[index].Key)
		}
		fmt.Println("-->", NatGatewayTagList)
		for index, PolicyTag := range PolicyTagList {
			fmt.Println("Policy Tag: ", index, " ", PolicyTag)
			TagCheck = contains(NatGatewayTagList, PolicyTag)
			if TagCheck == false {
				fmt.Println("TagCheck Failed.")
				UnTaggedNatGateway = append(UnTaggedNatGateway, *NatGateway.NatGatewayId)
				break
			} else if len(PolicyTagList)-1 == index && TagCheck == true {
				fmt.Println("TagCheck Success.")
				TaggedNatGateway = append(TaggedNatGateway, *NatGateway.NatGatewayId)
			}
		}
		NatGatewayTagList = nil
	}
	return TaggedNatGateway, UnTaggedNatGateway
}

func NatGatewayInit(PolicyObject *Policy, sess *session.Session) {
	svc := ec2.New(sess)
	input := ec2.DescribeNatGatewaysInput{}
	result, err := svc.DescribeNatGateways(&input)
	if err != nil {
		fmt.Print("Unable to load NatGateways %v", err)
	}

	if len(result.NatGateways) == 0 {
		fmt.Printf("No NatGateways for %s region\n", *svc.Config.Region)
	} else {
		fmt.Println(result.NatGateways)
		Tagged, UnTagged := NatGatewayFinder(svc, result.NatGateways, GetPolicyKeys(PolicyObject))
		fmt.Println("\n\n\n\nFinal Tagged:", Tagged)
		fmt.Println("Final UnTagged:", UnTagged)
	}
}

func NetworkAclFinder(svc *ec2.EC2, NetworkAclList []*ec2.NetworkAcl, PolicyTagList []string) ([]string, []string) {
	TagCheck := true
	fmt.Println("NetworkAcl:")
	var NetworkAclTagList []string
	var TaggedNetworkAcl, UnTaggedNetworkAcl []string
	for _, NetworkAcl := range NetworkAclList {
		for index, _ := range NetworkAcl.Tags {
			NetworkAclTagList = append(NetworkAclTagList, *NetworkAcl.Tags[index].Key)
		}
		fmt.Println("-->", NetworkAclTagList)
		for index, PolicyTag := range PolicyTagList {
			fmt.Println("Policy Tag: ", index, " ", PolicyTag)
			TagCheck = contains(NetworkAclTagList, PolicyTag)
			if TagCheck == false {
				fmt.Println("TagCheck Failed.")
				UnTaggedNetworkAcl = append(UnTaggedNetworkAcl, *NetworkAcl.NetworkAclId)
				break
			} else if len(PolicyTagList)-1 == index && TagCheck == true {
				fmt.Println("TagCheck Success.")
				TaggedNetworkAcl = append(TaggedNetworkAcl, *NetworkAcl.NetworkAclId)
			}
		}
		NetworkAclTagList = nil
	}
	return TaggedNetworkAcl, UnTaggedNetworkAcl
}

func NetworkAclInit(PolicyObject *Policy, sess *session.Session) {
	svc := ec2.New(sess)
	input := ec2.DescribeNetworkAclsInput{}
	result, err := svc.DescribeNetworkAcls(&input)
	if err != nil {
		fmt.Print("Unable to load NACL %v", err)
	}

	if len(result.NetworkAcls) == 0 {
		fmt.Printf("No NetworkAcls for %s region\n", *svc.Config.Region)
	} else {
		fmt.Println(result.NetworkAcls)
		Tagged, UnTagged := NetworkAclFinder(svc, result.NetworkAcls, GetPolicyKeys(PolicyObject))
		fmt.Println("\n\n\n\nFinal Tagged:", Tagged)
		fmt.Println("Final UnTagged:", UnTagged)
	}
}

func ReservedInstanceFinder(svc *ec2.EC2, ReservedInstanceList []*ec2.ReservedInstances, PolicyTagList []string) ([]string, []string) {
	TagCheck := true
	fmt.Println("ReservedInstance:")
	var ReservedInstanceTagList []string
	var TaggedReservedInstance, UnTaggedReservedInstance []string
	for _, ReservedInstance := range ReservedInstanceList {
		for index, _ := range ReservedInstance.Tags {
			ReservedInstanceTagList = append(ReservedInstanceTagList, *ReservedInstance.Tags[index].Key)
		}
		fmt.Println("-->", ReservedInstanceTagList)
		for index, PolicyTag := range PolicyTagList {
			fmt.Println("Policy Tag: ", index, " ", PolicyTag)
			TagCheck = contains(ReservedInstanceTagList, PolicyTag)
			if TagCheck == false {
				fmt.Println("TagCheck Failed.")
				UnTaggedReservedInstance = append(UnTaggedReservedInstance, *ReservedInstance.ReservedInstancesId)
				break
			} else if len(PolicyTagList)-1 == index && TagCheck == true {
				fmt.Println("TagCheck Success.")
				TaggedReservedInstance = append(TaggedReservedInstance, *ReservedInstance.ReservedInstancesId)
			}
		}
		ReservedInstanceTagList = nil
	}
	return TaggedReservedInstance, UnTaggedReservedInstance
}

func ReservedInstanceInit(PolicyObject *Policy, sess *session.Session) {
	svc := ec2.New(sess)
	input := ec2.DescribeReservedInstancesInput{}
	result, err := svc.DescribeReservedInstances(&input)
	if err != nil {
		fmt.Print("Unable to load Reserved Instances %v", err)
	}

	if len(result.ReservedInstances) == 0 {
		fmt.Printf("No Reserved Instances for %s region\n", *svc.Config.Region)
	} else {
		Tagged, UnTagged := ReservedInstanceFinder(svc, result.ReservedInstances, GetPolicyKeys(PolicyObject))
		fmt.Println("\n\n\n\nFinal Tagged:", Tagged)
		fmt.Println("Final UnTagged:", UnTagged)
	}
}

func RouteTableInit(PolicyObject *Policy, sess *session.Session) {
	svc := ec2.New(sess)
	input := ec2.DescribeRouteTablesInput{}
	result, err := svc.DescribeRouteTables(&input)
	if err != nil {
		fmt.Print("Unable to load Reserved Instances %v", err)
	}

	if len(result.RouteTables) == 0 {
		fmt.Printf("No Reserved Instances for %s region\n", *svc.Config.Region)
	} else {
		Tagged, UnTagged := RouteTableFinder(svc, result.RouteTables, GetPolicyKeys(PolicyObject))
		fmt.Println("\n\n\n\nFinal Tagged:", Tagged)
		fmt.Println("Final UnTagged:", UnTagged)
	}
}

func RouteTableFinder(svc *ec2.EC2, RouteTableList []*ec2.RouteTable, PolicyTagList []string) ([]string, []string) {
	TagCheck := true
	fmt.Println("Route Table:")
	var RouteTableTagList []string
	var TaggedRouteTable, UnTaggedRouteTable []string
	for _, RouteTable := range RouteTableList {
		for index, _ := range RouteTable.Tags {
			RouteTableTagList = append(RouteTableTagList, *RouteTable.Tags[index].Key)
		}
		fmt.Println("-->", RouteTableTagList)
		for index, PolicyTag := range PolicyTagList {
			fmt.Println("Policy Tag: ", index, " ", PolicyTag)
			TagCheck = contains(RouteTableTagList, PolicyTag)
			if TagCheck == false {
				fmt.Println("TagCheck Failed.")
				UnTaggedRouteTable = append(UnTaggedRouteTable, *RouteTable.RouteTableId)
				break
			} else if len(PolicyTagList)-1 == index && TagCheck == true {
				fmt.Println("TagCheck Success.")
				TaggedRouteTable = append(TaggedRouteTable, *RouteTable.RouteTableId)
			}
		}
		RouteTableTagList = nil
	}
	return TaggedRouteTable, UnTaggedRouteTable
}

// func SecurityGroupFinder(svc *ec2.EC2, SecurityGroupList []*ec2.SecurityGroupReference, PolicyTagList []string) ([]string, []string) {
// 	TagCheck := true
// 	fmt.Println("SecurityGroup:")
// 	var SecurityGroupTagList []string
// 	var TaggedSecurityGroup, UnTaggedSecurityGroup []string
// 	for _, SecurityGroup := range SecurityGroupList {
// 		for index, _ := range SecurityGroup. {
// 			RouteTableTagList = append(RouteTableTagList, *RouteTable.Tags[index].Key)
// 		}
// 		fmt.Println("-->", RouteTableTagList)
// 		for index, PolicyTag := range PolicyTagList {
// 			fmt.Println("Policy Tag: ", index, " ", PolicyTag)
// 			TagCheck = contains(RouteTableTagList, PolicyTag)
// 			if TagCheck == false {
// 				fmt.Println("TagCheck Failed.")
// 				UnTaggedRouteTable = append(UnTaggedRouteTable, *RouteTable.RouteTableId)
// 				break
// 			} else if len(PolicyTagList)-1 == index && TagCheck == true {
// 				fmt.Println("TagCheck Success.")
// 				TaggedRouteTable = append(TaggedRouteTable, *RouteTable.RouteTableId)
// 			}
// 		}
// 		RouteTableTagList = nil
// 	}
// 	return TaggedRouteTable, UnTaggedRouteTable
// }

func SecurityGroupReferenceInit(PolicyObject *Policy, sess *session.Session, SecurityGroupRulesList []*ec2.SecurityGroupRule) {
	svc := ec2.New(sess)
	var SecurityGroupRuleNamesList []*string
	for _, SecurityGroupRule := range SecurityGroupRulesList {
		fmt.Println()
		SecurityGroupRuleNamesList = append(SecurityGroupRuleNamesList, SecurityGroupRule.ReferencedGroupInfo.GroupId)
		input := ec2.DescribeSecurityGroupReferencesInput{
			GroupId: SecurityGroupRuleNamesList,
		}
		result, err := svc.DescribeSecurityGroupReferences(&input)
		if err != nil {
			fmt.Print("Unable to load SecurityGroup %v", err)
		}
		if len(result.SecurityGroupReferenceSet) == 0 {
			fmt.Printf("No Reserved Instances for %s region\n", *svc.Config.Region)
		} else {
			fmt.Println(result.SecurityGroupReferenceSet)
			// Tagged, UnTagged := RouteTableFinder(svc, result.SecurityGroupReferenceSet, GetPolicyKeys(PolicyObject))
			// fmt.Println("\n\n\n\nFinal Tagged:", Tagged)
			// fmt.Println("Final UnTagged:", UnTagged)
		}
	}
}

func SecurityGroupFinder(svc *ec2.EC2, SnapshotList []*ec2.Snapshot, PolicyTagList []string) ([]string, []string) {
	TagCheck := true
	fmt.Println("Snapshot:")
	var SnapshotTagList []string
	var TaggedSnapshot, UnTaggedSnapshot []string
	for _, Snapshot := range SnapshotList {
		for index, _ := range Snapshot.Tags {
			SnapshotTagList = append(SnapshotTagList, *Snapshot.Tags[index].Key)
		}
		for index, PolicyTag := range PolicyTagList {
			fmt.Println("Policy Tag: ", index, " ", PolicyTag)
			TagCheck = contains(SnapshotTagList, PolicyTag)
			if TagCheck == false {
				fmt.Println("TagCheck Failed.")
				UnTaggedSnapshot = append(UnTaggedSnapshot, *Snapshot.DataEncryptionKeyId)
				break
			} else if len(PolicyTagList)-1 == index && TagCheck == true {
				fmt.Println("TagCheck Success.")
				TaggedSnapshot = append(TaggedSnapshot, *Snapshot.DataEncryptionKeyId)
			}
		}
		SnapshotTagList = nil
	}
	return TaggedSnapshot, UnTaggedSnapshot
}

func SecurityGroupInit(PolicyObject *Policy, sess *session.Session) {
	svc := ec2.New(sess)
	input := ec2.DescribeSecurityGroupRulesInput{}
	result, err := svc.DescribeSecurityGroupRules(&input)
	if err != nil {
		fmt.Print("Unable to load SecurityGroupRules %v", err)
	}
	if len(result.SecurityGroupRules) == 0 {
		fmt.Printf("No Security Group for %s region\n", *svc.Config.Region)
	} else {
		// Tagged, UnTagged := SecurityGroupFinder(svc, result.SecurityGroupRules, GetPolicyKeys(PolicyObject))
		// fmt.Println("\n\n\n\nFinal Tagged:", Tagged)
		// fmt.Println("Final UnTagged:", UnTagged)
	}
}

func EC2SnapShotInit(PolicyObject *Policy, sess *session.Session) {
	svc := ec2.New(sess)
	input := ec2.DescribeSnapshotsInput{}
	result, err := svc.DescribeSnapshots(&input)
	if err != nil {
		fmt.Print("Unable to load SnapShot %v", err)
	}
	if len(result.Snapshots) == 0 {
		fmt.Printf("No Snapshots for %s region\n", *svc.Config.Region)
	} else {
		Tagged, UnTagged := SecurityGroupFinder(svc, result.Snapshots, GetPolicyKeys(PolicyObject))
		fmt.Println("\n\n\n\nFinal Tagged:", Tagged)
		fmt.Println("Final UnTagged:", UnTagged)
	}
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
	// LambdaInit(PolicyObject, sess)
	// RDSInit(PolicyObject, sess)
	// Route53Init(PolicyObject, sess)
	// SQSInit(PolicyObject, sess)
	// WorkspacesInit(PolicyObject, sess)
	// ElasticIpInit(PolicyObject, sess)
	// AmiInit(PolicyObject, sess)
	// InternetGatewayInit(PolicyObject, sess)
	// NatGatewayInit(PolicyObject, sess)
	// NetworkAclInit(PolicyObject, sess)
	// ReservedInstanceInit(PolicyObject, sess)
	// RouteTableInit(PolicyObject, sess)
	// SecurityGroupInit(PolicyObject, sess)
	EC2SnapShotInit(PolicyObject, sess)
}
