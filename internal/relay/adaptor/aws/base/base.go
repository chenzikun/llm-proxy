package base

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/zicorn/llm-proxy/pkg/common/logger"
	"github.com/zicorn/llm-proxy/internal/repo"
)

var AwsClientWithIAM *AwsClient

func init() {
	AwsClientWithIAM = &AwsClient{
		IsInitialized: false,
	}
}

type AwsClient struct {
	IsInitialized         bool
	AwsBedrockClient      *bedrockruntime.Client
	AwsBedrockAgentClient *bedrockagentruntime.Client
}

func GetOrCreateAwsClient(channelConfig *model.ChannelConfig) (*AwsClient, error) {
	if channelConfig.AK == "" {
		if !AwsClientWithIAM.IsInitialized {
			if err := AwsClientWithIAM.Init(channelConfig); err != nil {
				return nil, err
			}
			AwsClientWithIAM.IsInitialized = true
		}
		return AwsClientWithIAM, nil
	}
	awsClient := &AwsClient{
		IsInitialized: false,
	}
	if err := awsClient.Init(channelConfig); err != nil {
		return nil, err
	}
	return awsClient, nil
}

func (a *AwsClient) Init(channelConfig *model.ChannelConfig) error {
	if a.IsInitialized {
		return nil
	}
	if os.Getenv("OCI_2_AWS_ROLE_ARN") == "" && channelConfig.AK == "" && channelConfig.SK == "" {
		bedrockCfg, err := awsConfig.LoadDefaultConfig(
			context.TODO(),
			//awsConfig.WithRegion(channelConfig.GetRoleRegion()),
			awsConfig.WithRegion("us-west-2"),
		)
		if err != nil {
			return err
		}
		a.AwsBedrockClient = bedrockruntime.NewFromConfig(bedrockCfg)
		a.AwsBedrockAgentClient = bedrockagentruntime.NewFromConfig(bedrockCfg)
		return nil
	}
	staticProvider := credentials.NewStaticCredentialsProvider(channelConfig.AK, channelConfig.SK, "")
	if channelConfig.RoleARN == "" {
		a.AwsBedrockClient = bedrockruntime.New(bedrockruntime.Options{
			Region:      channelConfig.Region,
			Credentials: aws.NewCredentialsCache(staticProvider),
		})
		a.AwsBedrockAgentClient = bedrockagentruntime.New(bedrockagentruntime.Options{
			Region:      channelConfig.Region,
			Credentials: aws.NewCredentialsCache(staticProvider),
		})
		return nil
	}
	cfg, err := awsConfig.LoadDefaultConfig(context.TODO(),
		awsConfig.WithRegion(channelConfig.Region),
		awsConfig.WithCredentialsProvider(staticProvider))
	if err != nil {
		logger.SysLogf("unable to load AWS SDK config, %v", err)
		return err
	}

	// ===== assume role ======
	stsSvc := sts.NewFromConfig(cfg)

	// Assume role
	input := &sts.AssumeRoleInput{
		RoleArn:         aws.String(channelConfig.RoleARN),
		RoleSessionName: aws.String("bedrockruntime-session"),
	}
	result, err := stsSvc.AssumeRole(context.TODO(), input)
	if err != nil {
		return err
	}
	// Use assumed role to create new credentials
	assumedCreds := aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(
		*result.Credentials.AccessKeyId,
		*result.Credentials.SecretAccessKey,
		*result.Credentials.SessionToken,
	))

	// Create a BedrockRuntime client using the assumed role credentials
	bedrockCfg, err := awsConfig.LoadDefaultConfig(
		context.TODO(),
		awsConfig.WithRegion(channelConfig.GetRoleRegion()),
		awsConfig.WithCredentialsProvider(assumedCreds),
	)
	if err != nil {
		log.Printf("unable to create bedrock runtime with assummed role, %v", err)
		return err
	}

	a.AwsBedrockClient = bedrockruntime.NewFromConfig(bedrockCfg)
	a.AwsBedrockAgentClient = bedrockagentruntime.NewFromConfig(bedrockCfg)
	return nil
}
