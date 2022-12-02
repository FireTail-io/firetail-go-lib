# Petstore Chi Example

This project is a copy of the [oapi-codegen](https://github.com/deepmap/oapi-codegen) module's [petstore chi](https://github.com/deepmap/oapi-codegen/tree/master/examples/petstore-expanded/chi) example, which has had the oapi-codegen's request validation middleware replaced with the Firetail middleware.

Upon installing the Firetail middleware, several discrepancies were immediately discovered between the implementation and its corresponding [openapi 3.0 appspec](https://github.com/deepmap/oapi-codegen/blob/master/examples/petstore-expanded/petstore-expanded.yaml):

- The `GET /pets`, `POST /pets`, `GET /pets/{id}` and all error responses were missing their `Content-Type` headers. This is not mandatory, but it is good practice. This was rectified by adding the `Content-Type` headers to the responses.
- The `GET /pets` endpoint returned `null` when there were no pets in the store, but the openapi schema did not describe the response body as nullable. This was rectified by ammending the appspec to describe the response body as nullable.
- The `POST /pets` response code upon successful creation of a new pet was `201`, however the appspec defined a `200` response. This was rectified by ammending the appspec to define a `201` response, instead of a `200`, as `201` is a more apropriate status code in this case.

Some additional modifications have been made in order to demonstrate the Firetail middleware's functionalities, such as [Authentication](#authentication) and [Response Validation](#response-validation).



## Feature Overview

Using this familiar petstore example, we can demonstrate a number of the Firetail middleware's features:

1. [Request Validation](#request-validation)
2. [Authentication](#authentication)
3. [Response Validation](#response-validation)



### Request Validation

The Firetail middleware blocks:

- Requests made to paths that aren't defined in your appspec:
  ```bash
  curl localhost:8080/owners -v
  ```

  ```
  < HTTP/1.1 404 Not Found
  < Content-Type: application/json
  {"code":404,"title":"the resource \"/owners\" could not be found","debug":"a path for \"/owners\" could not be found in your appspec"}
  ```

- Requests made to paths that are defined in your appspec, but are made with unsupported methods:
  ```bash
  curl localhost:8080/pets -X DELETE -v
  ```

  ```
  < HTTP/1.1 405 Method Not Allowed
  < Content-Type: text/plain
  {"code":405,"title":"the resource \"/pets\" does not support the \"DELETE\" method","debug":"the path for \"/pets\" in your appspec does not support the method \"DELETE\""}
  ```

- Requests made to paths defined in your appspec, with methods defined in your appspec, but with a `Content-Type` that hasn't been defined in your appspec:
  ```bash
  curl localhost:8080/pets -X POST -H "Content-Type: application/xml" -d '<?xml version="1.0" encoding="UTF-8" ?><root><name>Hector</name></root>' -v
  ```

  ```
  < HTTP/1.1 415 Unsupported Media Type
  < Content-Type: text/plain
  {"code":415,"title":"the path for \"/pets\" in your appspec does not support the content type \"application/xml\"","debug":"the path for \"/pets\" in your appspec does not support content type \"application/xml\""}
  ```
  
- Requests made with path parameters that don't match the schema defined in your appspec:
  ```bash
  curl localhost:8080/pets/abc -v
  ```

  ```
  < HTTP/1.1 400 Bad Request
  < Content-Type: text/plain
  {"code":400,"title":"something's wrong with your path parameters","debug":"the request's path parameters did not match your appspec: parameter \"id\" in path has an error: value abc: an invalid integer: invalid syntax"}
  ```

- Requests made with query parameters that don't match the schema defined in your appspec:
  ```bash
  curl "localhost:8080/pets?limit=abc" -v
  ```

  ```
  < HTTP/1.1 400 Bad Request
  < Content-Type: text/plain
  {"code":400,"title":"something's wrong with your query parameters","debug":"the request's query parameters did not match your appspec: parameter \"limit\" in query has an error: value abc: an invalid integer: invalid syntax"}
  ```

- Requests made with bodies that don't match the schema defined in your appspec:

  ```bash
  curl localhost:8080/pets -X POST -H "Content-Type: application/json" -d '{"name":123}' -v
  ```

  ```
  < HTTP/1.1 400 Bad Request
  < Content-Type: text/plain
  {"code":400,"title":"something's wrong with your request body","debug":"the request's body did not match your appspec: request body has an error: doesn't match the schema: Error at \"/name\": Field must be set to string or not be present\nSchema:\n  {\n    \"description\": \"Name of the pet\",\n    \"type\": \"string\"\n  }\n\nValue:\n  \"number, integer\"\n"}
  ```



### Authentication

The petstore appspec has been modified to provide a `GET /auth` endpoint which provides a JWT, and apply the following security scheme to the `DELETE /pets/{id}` endpoint: 

```yaml
securitySchemes:
  MyBearerAuth:
    type: http
    scheme: bearer
    bearerFormat: JWT
```

The validation of this security scheme was implemented in the application by simply defining an `AuthCallback` and providing it to the Firetail middleware as follows:

```go
firetailMiddleware, err := firetail.GetMiddleware(&firetail.Options{
	OpenapiSpecPath: "./petstore-expanded.yaml",
  DebugErrs: true,
	AuthCallbacks: map[string]openapi3filter.AuthenticationFunc{
		"MyBearerAuth": func(ctx context.Context, ai *openapi3filter.AuthenticationInput) error {
			authHeaderValue := ai.RequestValidationInput.Request.Header.Get("Authorization")
			if authHeaderValue == "" {
				return errors.New("no bearer token supplied for \"MyBearerAuth\"")
			}

			tokenParts := strings.Split(authHeaderValue, " ")
			if len(tokenParts) != 2 || strings.ToUpper(tokenParts[0]) != "BEARER" {
				return errors.New("invalid value in Authorization header for \"MyBearerAuth\"")
			}

			token, err := jwt.Parse(tokenParts[1], func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
				}
				hmacSampleSecret := []byte(os.Getenv("JWT_SECRET_KEY"))
				return hmacSampleSecret, nil
			})

			if !token.Valid {
				return errors.New("invalid jwt supplied for \"MyBearerAuth\"")
			}

			return err
		},
	},
})
```

We can test this out by creating a pet:

```bash
curl localhost:8080/pets -X POST -H "Content-Type: application/json" -d '{"name":"Freya"}' -v
```

```
< HTTP/1.1 201 Created
< Content-Type: application/json
{"id":1000,"name":"Freya"}
```

Then attempting to:

1. Delete the pet without a JWT:
   ```bash
   curl localhost:8080/pets/1000 -X DELETE -v
   ```

   ```
   < HTTP/1.1 401 Unauthorized
   < Content-Type: text/plain
   {"code":401,"title":"you're not authorized to do this","debug":"the request did not satisfy the security requirements in your appspec: security requirements failed: no bearer token supplied for \"MyBearerAuth\", errors: no bearer token supplied for \"MyBearerAuth\""}
   ```

2. Delete the pet with an invalid JWT:
   ```bash
   curl localhost:8080/pets/1000 -X DELETE -H "Authorization: bearer header.payload.badsignature" -v
   ```

   ```
   < HTTP/1.1 401 Unauthorized
   < Content-Type: text/plain
   {"code":401,"title":"you're not authorized to do this","debug":"the request did not satisfy the security requirements in your appspec: security requirements failed: invalid jwt supplied for \"MyBearerAuth\", errors: invalid jwt supplied for \"MyBearerAuth\""}
   ```

3. Get a valid JWT from the petstore's `/auth` endpoint:
   ```bash
   curl localhost:8080/auth -v
   ```

   ```
   < HTTP/1.1 200 OK
   < Content-Type: application/json
   {"token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpYXQiOjE2Njk5ODA5MzR9.qDqP_r2hmrrGE3mWJbKoYK_Agz76Q57WntdGHfYc1F8"}
   ```

4. Delete the pet with a valid JWT:

   ```bash
   curl localhost:8080/pets/1000 -X DELETE -H "Authorization: bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpYXQiOjE2Njk5ODA5MzR9.qDqP_r2hmrrGE3mWJbKoYK_Agz76Q57WntdGHfYc1F8" -v
   ```

   ```
   < HTTP/1.1 204 No Content
   ```

   After which there should be no pets:

   ```bash
   curl localhost:8080/pets -v
   ```

   ```
   < HTTP/1.1 200 OK
   < Content-Type: application/json
   null
   ```



### Response Validation

The Firetail middleware will also validate the responses generated by your application to ensure they conform to the OpenAPI specification. If they do not, then a `500` response will be returned to the client instead.

Response validation is only applied to the responses generated by your application before the Firetail middleware - the Firetail middleware does not validate its own responses against the openapi spec, as this could lead to behaviours where your API will never pass its own validation, and so never return. This discrepancy is an issue we are currently exploring a number of possible solutions for.



#### Response Code Validation

The default response for the `POST /pet` endpoint has been removed from the appspec, and the implementation modified such that when it is attempted to create a pet with a name of `""`, the empty string, the application returns a `400`. This allows us to test the scenario where the application returns a response code that is not defined in the appspec:

```bash
curl localhost:8080/pets -X POST -H "Content-Type: application/json" -d '{"name":""}' -v
```

```
< HTTP/1.1 500 Internal Server Error
< Content-Type: application/json
{"code":500,"title":"internal server error","debug":"the response's status code did not match your appspec: 400"}
```



#### Response Header Validation

Response headers are not currently validated against your appspec, but will be in a future release.



#### Response Body Validation

This petstore example's appspec has been modified such that the `GET /pets` response body should only include the `name` and `id` of each pet, with no `owner`. The following definition was added to the appspec's schema definitions:

```yaml
NamedPet:
  required:
    - id
    - name
  additionalProperties: false
  properties:
    id:
      type: integer
      format: int64
      description: Unique id of the pet
    name:
      type: string
      description: Name of the pet
```

And the `GET /pets` response was updated to use the above definition:

```yaml
'200':
  description: pet response
  content:
    application/json:
      schema:
        type: array
        nullable: true
        items:
          $ref: '#/components/schemas/NamedPet'
```

This allows us to test out the response body validation by:

1. Creating a pet with an owner:

   ```bash
   curl localhost:8080/pets -X POST -H "Content-Type: application/json" -d '{"name":"Spot","owner":"Data"}' -v
   ```

   ```
   < HTTP/1.1 201 Created
   < Content-Type: application/json
   {"id":1000,"name":"Spot","owner":"Data"}
   ```

2. Making a `GET` request to `/pets`:
   ```bash
   curl localhost:8080/pets -v
   ```

   ```
   < HTTP/1.1 500 Internal Server Error
   < Content-Type: text/plain
   {"code":500,"title":"internal server error","debug":"the response's body did not match your appspec: response body doesn't match the schema: Error at \"/0\": property \"owner\" is unsupported\nSchema:\n  {\n    \"additionalProperties\": false,\n    \"properties\": {\n      \"id\": {\n        \"description\": \"Unique id of the pet\",\n        \"format\": \"int64\",\n        \"type\": \"integer\"\n      },\n      \"name\": {\n        \"description\": \"Name of the pet\",\n        \"type\": \"string\"\n      }\n    },\n    \"required\": [\n      \"id\",\n      \"name\"\n    ]\n  }\n\nValue:\n  {\n    \"id\": 1000,\n    \"name\": \"Spot\",\n    \"owner\": \"Data\"\n  }\n"}
   ```
   
   