# prometheus-fast-remote

Prometheus Remote Adapter


Supported TSDB :
* KairosDB

## Usage

### Direct binary

1. Download latest binary at https://github.com/orange-cloudfoundry/prometheus-fast-remote/releases 
2. Create a `config.yml` (see [example](/config.yml))
3. Run it: `./prometheus-fast-remote -config ./config.yml`

### Docker image

#### Environment variable

`docker run -e kairos_url=https://kairos.com -d orangeopensource/prometheus-fast-remote`

**Tip**: You can see other environment variable in [launch.sh](/launch.sh) file.

#### Config file

1. Create a `config.yml` (see [example](/config.yml))
2. run `docker run -v ./config.yml:/config.yml -d orangeopensource/prometheus-fast-remote`


## Api

### Read

Read implementation for prometheus remote adapter

-**Path**: `/read`
-**Method**: `GET`

### Write

Write implementation for prometheus remote adapter

-**Path**: `/write`
-**Method**: `POST`

### Health

Checks the status of each health check. 
If all are healthy it returns status 200 otherwise it returns 500.

-**Path**: `/health`
-**Method**: `GET`
-**Response code**:
  - *success*: `200`
  - *Failure*: `500`
-**Response body** (example):
```json
{
  "adapter": "ok",
  "tsdb": {
    "name": "kairosdb",
    "status": "ok"
  }
}
```