create table vault (
  uuid uuid,
  url text,
  height int,
  width int,
  primary key(uuid, url)
)
