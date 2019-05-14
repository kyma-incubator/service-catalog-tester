# OSB checker

We use [this](https://github.com/openservicebrokerapi/osb-checker) project to check the OSB contract in our brokers.

## Usage

Use the dockerfile to build an image and use it to test your broker.

`docker build . -t osb-checker/test-job:2.13`

An application created from the dockerfile calls the broker under the `localhost` so the best way to use it is to just add this image to the broker's container.

To do it just add the following code into your `deployment.yaml`:

```yaml
  - name: "tester"
    image: "osb-checker/test-job:2.13"
    imagePullPolicy: Never
    command: ["/bin/sh"]
    args: ["./test.sh", "localhost", "8080", "150"]
```

Now the test is able to contact the broker and test the OSB contract.

## Configuration

You can change the test configuration in the `config_mock.json` file.

Currently the `config_mock.json` file is configured to test the `Helm Broker`.

>**NOTE:** The test has cases which our brokers don't implement, like update action or binding conflicts, so the test returns some errors anyway.
