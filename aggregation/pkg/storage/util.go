package storage

import "github.com/georgemblack/blue-report/pkg/app/util"

func siteBucketName() string {
	return util.GetEnvStr("S3_BUCKET_NAME", "blue-report-test")
}

func readAssetsBucketName() string {
	return util.GetEnvStr("S3_ASSETS_BUCKET_NAME", "blue-report-assets")
}

func writeAssetsBucketName() string {
	return util.GetEnvStr("S3_ASSETS_BUCKET_NAME", "blue-report-test")
}

func urlMetadataTableName() string {
	return util.GetEnvStr("DYNAMO_URL_METADATA_TABLE", "blue-report-url-metadata-test")
}
