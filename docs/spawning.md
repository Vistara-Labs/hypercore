Hypercore supports deployments of any containerized application, regardless of the programming language/frameworks used. All we need to do is dockerize our application, build the image, and push it to a Docker registry. Then, the image can be deployed by Hypercore and be exposed to the public network by the in-built reverse proxy.

## Deploying a NextJS application

As an example, we'll be deploying a [sample NextJS app](https://github.com/vercel/next.js/tree/canary/examples/with-docker) with Hypercore

1. Setup the sample app using `npx`

```bash
$ npx create-next-app --example with-docker nextjs-docker
```

1. Build the Docker image, passing Vistara's pre-hosted Docker registry in the tag

```bash
$ cd nextjs-docker
$ docker build -t registry.vistara.dev/next-example:latest .
```

1. Push the Docker image to the registry

```bash
$ docker push registry.vistara.dev/next-example:latest
```

1. Send a deployment request to Hypercore, specifying the image tag and the ports we want to expose. The sample application exposes itself at port `3000`, so we'll be mapping port `443` to port `3000` on the container

```bash
# Replace MY_HYPERCORE_IP with the IP address of the deployed hypercore
$ hypercore cluster spawn \
    --grpc-bind-addr "$MY_HYPERCORE_IP:8000" \
    --ports 443:3000 \
    --image-ref registry.vistara.dev/next-example:latest
INFO[0010] Got response: id:"06d0f10a-a6c6-45ae-8f23-770b96851bc3"  url:"06d0f10a-a6c6-45ae-8f23-770b96851bc3.deployments.vistara.dev"
```

The image is now deployed and can be accessed at the returned URL `06d0f10a-a6c6-45ae-8f23-770b96851bc3.deployments.vistara.dev`

![image](https://github.com/user-attachments/assets/be79591c-fc61-4285-ac0d-c44f805ba47e)
