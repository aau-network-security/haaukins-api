# Haaukins API

Haaukins API allows the user to connect directly Kali Linux VM avoiding normal Haaukins steps. Unlike Haaukins, in which
you need to have an event up and running in order to sign up and connect Kali Linux VM, on the API you just need to select
the challenges you want to have in your Environment and run it. In almost a minute the environment will be ready and the 
user will be automatically redirect to Guacamole that render the Kali linux VM.

### Keywords used on this repository
- Client: Identify (through session cookie containing an ID) who made the request on the API
- Environment: Identify the challenges requested from the Client. It contantains a guacamole instance, a lab and a timer.
- Challenges: They are Haaukins Exercises

The relation in between those 3 keywords is the following 
> Client 1 ----> N Environment 1 ----> M Challenges

### API implementation
The API has some constrains in order to don't create too many Environment and 
- the API has a Max amount of requests that can handle (specify the value on the config file)
- a Client has a Max amount of requests that can make
- the Environment will be closed after 45 minutes it has been created thanks the timer.

In case a Client or the API reached the Max amount of request, the Clients have to wait that at the least an Environment
will be closed in order to make another request

### Configuration file
the API needs a configuration file in order to works. It should look like the following:
```yaml
host: # host
port:
  insecure: 80
  secure: 443
tls: # certificates absolute path
  enabled: true
  certfile: 
  certkey: 
  cafile: 
exercises-file: # string, absolute path to the exercise.yml file
ova-dir: # directory where to pull the .ova images
api:
  sign-key: # sign key for the session cookie
  admin: # basic auth in order to get the list of Environment running
    username: 
    password: 
  captcha: # re-captcha information
    enabled: true
    site-key: 
    secret-key: 
  total-max-requests: # int, number of request the API can handle
  client-max-requests: # int, number request a client can make
  frontend:
    image: kali
    memory: 4096
  store-file: # string, .csv file where to store the requests 
docker-repositories: # Gitlab registry
  - username: 
    password: 
    serveraddress: 
```

### How it works (for developers)

When the API receives a request under this path `/api/`, it passes through a middleware that makes some check and initialise some variable.
The following schema explain how the middleware works.

<img src=".github/imgs/api_workflow.png"  />

1. API handle the request and checks (shows Error Page in case of error):
    - if the challenges TAG selected exists
    - if the API can handle another request
    - if the user is not a BOT through a reCAPTCHA
2. API check for a session cookie in order to check is a Client exists:
    - if exists it means a Client already made at the least a request, so the request is forwarded to step 3.
    - if not a new Client is created, new session cookie send as response and new Environment created (4)
3. Get the Client from the session cookie and:
    - check if the requested challenges are already running in an environment, if so redirect the Client to Kali Linux
    - if not create new Environment
