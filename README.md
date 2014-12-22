go-flow-s3
==============

This server implements the streaming [flow.js](https://github.com/flowjs/flow.js)
Javascript upload API. Read the full flow.js library for details.

Once a file is successfully and completely uploaded, this library will put the file
contents into a S3 bucket.

The name of the file will be the hex digest from the image bytes being fed through
sha256 and the extension of the uploaded file.

The mime type will be determined from the file extension.

We are using the [mitchellh/amz](https://github.com/mitchellh/goamz) so follow that
repo's recommendation for the AWS credentials. The easiest way is to provide
the `AWS_ACCESS_KEY_ID` and the `AWS_SECRET_ACCESS_KEY` in your environment.

go-flow-s3 attempts to limit scope as much as possible. Therefore, you must provide
the `BUCKETNAME` in your env. The AWS credentials for `mitchellh/amz` must have
GET, PUT, and DELETE permissions for your particular `S3_BUCKET`.

This micro service is running on a Dokku instance, but could easily be run on a
Heroku Dyno or your own server.

**If** `SKIP_S3_UPLOAD` is a truthy value, images will not be put into your S3 bucket.
Behind the scenes, this service is using [boltdb](https://github.com/boltdb/bolt) which
is a low level key value store that uses files. This service uses three different bolt
databases. You must define four environment variables, each indicating where you
want the particular database file. Make sure the directories exist, but the actual
file doesn't have to. The Bolt client will create the file if necessary.

* `BOLT_IMAGES` - Local Image Storage
* `BOLT_URLS` - Local Url lists
* `BOLT_CHUNKS` - Temporary chunks for flow files
* `BASE_PATH` - host you are running on `http://localhost:3000`

###Why?

All of the flow server examples were just examples really and didn't work as intended.
Since these days, who wants to actually save files, pipeing them to S3 seems cool.
