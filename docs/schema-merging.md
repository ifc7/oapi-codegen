# How `allOf`, `anyOf` and `oneOf` become Go types

OpenAPI composes schemas with `allOf` (a value must match every schema listed), `anyOf` (at least one) and `oneOf` (exactly one). Go has no composition, so `oapi-codegen` has to compute an appropriate Go model for each:

- An `allOf` is merged into one Go type, by what `allOf` means: a value matches every member. A disagreement no value can satisfy is reported as an error that names both members.
- An `anyOf` or `oneOf` becomes a union type. A union type holds the JSON value, and has, for each variant, `As*` to read the value as that variant, `From*` to set it, and `Merge*` to merge a variant into it. A discriminated union also has `Discriminator()` and `ValueByDiscriminator()`.

A number of common idioms are read the way their authors meant them; see [Where the spec is read generously](#where-the-spec-is-read-generously).

- [`allOf`](#allof)
- [`anyOf` and `oneOf`](#anyof-and-oneof)
- [Where the spec is read generously](#where-the-spec-is-read-generously)

## `allOf`

| Keyword in several members | Merged as |
|---|---|
| `properties` | merged, [see below](#a-property-declared-in-several-members) |
| `type` | the types they have in common: `integer` narrows `number`. No common type is an error |
| `enum` | the values they have in common, [see below](#enums) |
| `const` | must be the same, and allowed by the `enum` |
| `required` | all of them |
| `items`, `additionalProperties` | merged like properties |
| `discriminator` | the same one may repeat |
| a nested `allOf` member's own keywords | kept |

Descriptions, examples, validation constraints and nullability never conflict. A property, or the composition, is nullable when any member makes it so.

```yaml
components:
  schemas:
    Client:
      type: object
      required: [name]
      properties:
        name: {type: string}
    ClientWithId:
      allOf:
        - $ref: '#/components/schemas/Client'
        - properties:
            id: {type: integer}
          required: [id]
```

```go
type ClientWithId struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}
```

### A property declared in several members

A member can refine a property another member declares. This is common for patch payloads:

```yaml
Base:
  type: object
  required: [name]
  properties:
    name: {type: string}
Patch:
  allOf:
    - $ref: '#/components/schemas/Base'
    - properties:
        name: {nullable: true}
```

```go
type Patch struct {
	Name *string `json:"name"`
}
```

`Base`'s string, made nullable by the second member. The property is merged as an `allOf` of its declarations, so nested objects merge their properties the same way, and the declarations' descriptions and field-level extensions such as `x-go-name` are kept.

### Conflicts are errors

When members declare a property with types no value can have, generation fails:

```yaml
Base:
  type: object
  properties:
    age: {type: integer}
Person:
  allOf:
    - $ref: '#/components/schemas/Base'
    - properties:
        age: {type: string}
```

```
allOf can't merge #/components/schemas/Base/properties/age (type integer)
with allOf/1/properties/age (type string): no value has both types
```

Types that do overlap narrow instead: `allOf: [{type: number}, {type: integer}]` is an `int`.

### Enums

A value has to match every member, so the merge keeps the values the members' enums have in common:

```yaml
Color:
  type: string
  enum: [red, green, blue]
Cool:
  allOf:
    - $ref: '#/components/schemas/Color'
    - enum: [green, blue, violet]
```

```go
const (
	CoolBlue  Cool = "blue"
	CoolGreen Cool = "green"
)
```

Enums with no value in common are an error. Some specs use `allOf` on purpose to add values to an enum. The `x-oapi-codegen-enum-merge: union` extension asks for that:

```yaml
Status:
  type: string
  enum: [active, inactive]
ExtendedStatus:
  x-oapi-codegen-enum-merge: union
  allOf:
    - $ref: '#/components/schemas/Status'
    - enum: [archived]
```

This generates `active`, `inactive` and `archived`. The extension goes on the schema that has the `allOf`, or on a member's property that refines a property. Validators still read the `allOf` literally, so a service that validates requests against the spec has to accept the extra values some other way.

### An `allOf` that only annotates a `$ref`

OpenAPI 3.0 allows nothing next to a `$ref`, so specs wrap one in an `allOf` to add a description, or to make it nullable. Such an `allOf` generates the referenced type itself, rather than a copy of it:

```yaml
Named:
  type: object
  required: [name]
  properties:
    name: {type: string}
Described:
  allOf:
    - $ref: '#/components/schemas/Named'
    - description: A Named with a description of its own.
Holder:
  type: object
  properties:
    owner:
      allOf:
        - $ref: '#/components/schemas/Named'
        - nullable: true
```

```go
// Described A Named with a description of its own.
type Described = Named

type Holder struct {
	Owner *Named `json:"owner"`
}
```

A member only annotates when it has no keyword that shapes a Go type: documentation, `nullable` or a 3.1 `"null"` type, `readOnly`, `writeOnly`, `default`, validation constraints, and extensions other than `x-go-type` and the enum-naming `x-enum-varnames` and `x-enumNames`, which make a new type. A member that restates the referenced schema's type, like `{type: object, description: …}`, counts too. `x-go-type-skip-optional-pointer` in an annotating member applies to the result, and `x-go-type-name` names an alias. A member with `properties`, `required`, an `enum` or a different `type` makes a new type, merged as above.

### Schemas from other documents, and `x-go-type`

The merge doesn't read the schema of an `allOf` member that is in another document, or that `x-go-type` replaces with a Go type. Another generator run owns the external schema's type, and an `x-go-type`'s fields are unknown, so neither can be merged into a new struct:

- Such a member plus members that only annotate it is that member's type. A property `allOf: [$ref other.yaml#/User, {nullable: true}]` is `*externalRef0.User`, and a component with it is an alias, `= externalRef0.User`.
- Such a member plus any member that shapes a Go type is an error. For a schema from another document, define the schema in this one instead; for `x-go-type`, give the composition an `x-go-type` of its own.

```yaml
Timestamp:
  type: string
  x-go-type: time.Time
Created:
  allOf:
    - $ref: '#/components/schemas/Timestamp'
    - description: When it was created.
```

```go
// Created When it was created.
type Created = Timestamp
```

External references inside a member, in its properties, items or `additionalProperties`, are just type names, and work as anywhere else.

## `anyOf` and `oneOf`

### A list that only adds constraints

A `oneOf` or `anyOf` often says which properties must be present, rather than listing alternative types:

```yaml
Contact:
  type: object
  properties:
    email: {type: string}
    phone: {type: string}
  oneOf:
    - required: [email]
    - required: [phone]
```

```go
type Contact struct {
	Email *string `json:"email,omitempty"`
	Phone *string `json:"phone,omitempty"`
}
```

No union is generated for a list whose branches declare nothing that shapes a Go type besides `required` and a boolean `additionalProperties`, or only restate the schema's own type. A validator still enforces the constraint; the Go type just doesn't pretend it's a choice of types. A branch with a type of its own, a `$ref` into another document, `x-go-type` or `x-go-type-name` is a type, and the union stays. A local `$ref` branch is judged by the schema it refers to.

### A child that is an `allOf` of its parent

Some generators write inheritance with the parent listing its children in a `oneOf`, and each child an `allOf` of the parent:

```yaml
Pet:
  type: object
  required: [petType]
  properties:
    petType: {type: string}
  discriminator:
    propertyName: petType
  oneOf:
    - $ref: '#/components/schemas/Cat'
    - $ref: '#/components/schemas/Dog'
Cat:
  allOf:
    - $ref: '#/components/schemas/Pet'
    - properties:
        meow: {type: string}
Dog:
  allOf:
    - $ref: '#/components/schemas/Pet'
    - properties:
        bark: {type: string}
```

A `Cat` is already one of `Pet`'s branches, so it is a plain struct, and `Pet` is the union of its children:

```go
type Cat struct {
	Meow    *string `json:"meow,omitempty"`
	PetType string  `json:"petType"`
}
```

### An `allOf` of several unions

```yaml
Payment:
  oneOf:
    - $ref: '#/components/schemas/Card'
    - $ref: '#/components/schemas/Transfer'
Delivery:
  oneOf:
    - $ref: '#/components/schemas/Courier'
    - $ref: '#/components/schemas/Pickup'
Order:
  allOf:
    - $ref: '#/components/schemas/Payment'
    - $ref: '#/components/schemas/Delivery'
```

An `Order` is a payment and a delivery at once, so each union is kept: a variant's `From*` replaces only its own union's data.

```go
var o Order
o.FromCard(Card{Card: "4111"})
o.FromCourier(Courier{Address: "1 Main St"})
// {"address": "1 Main St", "card": "4111"}
o.FromTransfer(Transfer{Iban: "DE00"})
// {"address": "1 Main St", "iban": "DE00"}
```

A union that is a `$ref` also gets `As*` and `From*` of its own: `o.AsPayment()` returns a `Payment` without the delivery's data, and `o.FromPayment(p)` sets the payment and keeps the delivery. A key that variants of several unions declare, such as a shared `id`, is overwritten but never removed. A discriminator belongs to the union whose variants it tells apart. Inline variants of one of several unions are named `<Name>OneOf<n><index>` or `<Name>AnyOf<n><index>`, where `<n>` is only there when there are several `oneOf`s or `anyOf`s.

### Discriminated inline variants

A discriminator maps its values to schema names, explicitly with `mapping` or by each `$ref`'s name. An inline variant has no name, so it is mapped to the value it pins for the discriminator property, with a `const` or an `enum` of one value:

```yaml
Pet:
  oneOf:
    - type: object
      properties:
        petType: {type: string, enum: [cat]}
        meow: {type: string}
    - type: object
      properties:
        petType: {type: string, enum: [dog]}
        bark: {type: string}
  discriminator:
    propertyName: petType
```

`ValueByDiscriminator()` returns a `Pet0` for `"cat"` and a `Pet1` for `"dog"`, and `FromPet0` writes `"petType": "cat"`.

An inline variant that pins no value has no discriminator value: `ValueByDiscriminator()` doesn't lead to it, and its `From*` writes none, but its `As*` and `From*` work. Also:

- An `integer` or `number` discriminator is read as a number, so `1`, `1.0` and `1e0` of a `number` discriminator lead to the same variant.
- `From*` writes the discriminator value for each variant exactly one value leads to ([#2071](https://github.com/oapi-codegen/oapi-codegen/issues/2071)).

### Variants with `additionalProperties: false`

`As*` decodes the union's value into the variant. Decoding alone ignores the keys the variant doesn't declare, so it can't tell variants apart ([#668](https://github.com/oapi-codegen/oapi-codegen/issues/668)):

```yaml
Subject:
  oneOf:
    - $ref: '#/components/schemas/Base'
    - $ref: '#/components/schemas/Extended'
Base:
  type: object
  additionalProperties: false
  required: [id]
  properties:
    id: {type: string}
Extended:
  allOf:
    - $ref: '#/components/schemas/Base'
    - type: object
      required: [extra]
      properties:
        extra: {type: string}
```

So the `As*` of a variant with `additionalProperties: false` checks the value's top-level keys first: `AsBase()` of `{"id": "2", "extra": "more"}` returns the error `Base doesn't allow the property "extra"`. It allows the keys the variant declares, and those anything else in the object declares: the union's own properties, its discriminator, and, in an `allOf` of unions, the other unions' variants. Nested objects are decoded without the check. A variant whose keys can't all be known, because it is from another document, has `x-go-type`, or has a union of its own, isn't checked. Neither is any variant of a union in an `allOf` with another union that has such a variant, since that variant might declare any key.

To turn the check off, set:

```yaml
output-options:
  lenient-union-accessors: true
```

Clients may want this, so that they don't fail when a newer server adds a property to a variant.

## Where the spec is read generously

Some idioms are common in real specs, but read literally, they don't do what their authors meant, or reject every value. `oapi-codegen` generates what the author meant. OpenAPI validators, such as kin-openapi's `openapi3filter`, read them literally, so if you validate against your spec, use the spelling in the last column.

| Idiom | Literally | Generates | Spelling validators also accept |
|---|---|---|---|
| 3.0: `allOf: [$ref A, {nullable: true}]` | `null` must match `A` too, which rejects it | `*A` | `nullable: true` next to the `allOf`: `{nullable: true, allOf: [$ref A]}` |
| 3.1: `allOf: [$ref A, {type: 'null'}]`, or a member whose `type` includes `"null"` | `null` must match `A` too, and with `type: 'null'` nothing else matches, so no value does | `*A` | `anyOf: [$ref A, {type: 'null'}]` |
| `additionalProperties: false` in one `allOf` member | the member rejects the other members' properties, so no value with them matches | the whole object is closed | declare all the properties in the closed schema |
| a `oneOf` variant with `additionalProperties: false` | the variant rejects the union's own properties and discriminator | `As*` allows them | declare those properties in the variant too |
| `x-oapi-codegen-enum-merge: union` | the enums intersect | all the values | none: allow the extra values in your service |
