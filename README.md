# service-analyzer
Auto-analysis functionality for ReportPortal


### Configuration
The following should be added to the docker-compose descriptor

```yml
  analyzer-equals:
    image: reportportal/service-analyzer-equals:4.0.1
    environment:
      - RP_SERVER.PORT=8080
      - RP_CONSUL.TAGS=urlprefix-/analyzer-equals strip=/analyzer-equals analyzer=equals analyzer_priority=1 analyzer_index=true
      - RP_CONSUL.ADDRESS=registry:8500
      - RP_REGISTRY=consul
    depends_on:
       - registry
       - elasticsearch
    restart: always
```
Please, pay attention to the following consul tags:
- analyzer_priority=1 Specifies priority of analyzer. Higher value means less priority. Notice that default value is 10.
- analyzer=equals Name of analyzer so you can find its actions in activity widget
- analyzer_index=true Analyzer supports indexing (accepts indexing requests)