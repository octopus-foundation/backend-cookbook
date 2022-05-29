# Relation database layout

One of the most underestimated problems in database structure design is maintainability. 
Projects can be in development for years, and at some point database structure will become incompatible with new requirements. 
At this point, making changes becomes very complicated due to the large amount of data and high concurrency. 
In other words, you cannot just use alters and lock tables, you need to "cheat".
For example, by using new tables (`users_2`, `users_3` - one of nightmare examples) and moving data between them.

The ideal design should be:

- simple - easy to extend
- easy to query
- maintainable
- independent of features
- independent of application
- maintainable in runtime (regardless of how much time has passed)
- maintainable by multiple developers
- fast for query execution


# Let's merge relational with key-value

One solution we've found is to mix relational databases with the key-value store approach. The idea is simple, 
you can create one table for entities and a key-value table with all fields related to them.
Let's apply this approach for the most common case, `users` table. In our concept, it will look like this:

```mysql
create table users
(
    id         bigint unsigned not null primary key auto_increment,
    created_at timestamp default now()
)
```

There are no `name`, `email`, `password hash`,  etc. There is only user id and nothing more. So how can we add properties to user? 
We just need key-value table for them:

```mysql
create table users_opts
(
    id             bigint unsigned not null primary key auto_increment,
    user_id        bigint unsigned not null,
    opt_type       bigint unsigned not null,

    opt_value_str  varchar(255),
    opt_value_uint bigint unsigned,
    opt_value_bool boolean,
    opt_value_blob mediumblob,

    unique key (user_id, opt_type),
    key (user_id, opt_type, opt_value_str) # for unique-related queries, e.g. check is phone used 
)
```

But how do we maintain opt types and be sure of their ids? There are two ways:

1. Describe them in a protobuf and share this protobuf across projects:

```protobuf
enum UserOptType {
  UOT_EMAIL = 0;
  UOT_NAME = 1;
  // ...
}

enum UserOptValueType {
  UOVT_STRING = 0;
  UOVT_UINT = 1;
  UOVT_BOOL = 2;
  UOVT_BLOB = 3;
}
```

2. Create a specific table in the database:

```mysql
 create table users_opts_types
 (
     opt_type   bigint unsigned not null primary key,
     name       varchar(255)    not null,
     value_type int unsigned    not null
 );
```

But you should not create multiple sources of truth! So `users_opts_types` should always be automatically updated 
by the backend to be consistent with the protobuf structure.

Imagine that you need to add a new "field" to `users`. You just add it to protobuf and that's all. 
You don't need any migration to modify the `users` table, and you can add as many "opts" as you need without it being a pain.

# Querying data
Let's find out how we can query this data. It's very simple and fast, just use left joins:
```mysql
select 
  name_opt.opt_value_str as name,
  email_opt.opt_value_str as email
from users
left join users_opts as name_opt on name_opt.user_id = users.id and name_opt.opt_type = ?
left join users_opts as email_opt on email_opt.user_id = users.id and email_opt.opt_type = ?
where users.id = ?;
```

Why `?`  instead of amounts of opt types? Because you need to know where your opt types are used. 
Just pass enum values on query execution, and after that you will always be able to find all usages of specific opts.

# Use sql files rather than direct sql code usage
TBA

# Migration hell
TBA

# Don't use ORM
TBA

# TODO
- N+1 pages, performance hit, data locality (cache miss count on this case)