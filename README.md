go-flow-s3
==============

This server implements the streaming [flow.js](https://github.com/flowjs/flow.js)
Javascript upload API. Read the full flow.js library for details.

Once a file is successfully and completely uploaded, this library will put the file
contents into a S3 bucket.

We are using the [mitchellh/amz](https://github.com/mitchellh/goamz) so follow that
repo's recommendation for the AWS credentials.

go-flow-s3 attempts to limit scope as much as possible. Therefore, you must provide
the `BUCKETNAME` in your env. The AWS credentials for `mitchellh/amz` must have
GET, PUT, and DELETE permissions for your particular `BUCKETNAME`.
