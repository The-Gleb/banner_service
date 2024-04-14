CREATE TABLE "banners" (
  "id" bigserial PRIMARY KEY,
  "title" varchar,
  "text" text,
  "url" varchar,
  "is_active" boolean,
  "created_at" timestamp,
  "updated_at" timestamp
);

CREATE TABLE "tags" (
  "id" bigserial PRIMARY KEY
);

CREATE TABLE "features" (
  "id" bigserial PRIMARY KEY
);

CREATE TABLE "banner_tag" (
  "banner_id" bigserial,
  "tag_id" bigserial,
  UNIQUE ("banner_id", "tag_id")
);

CREATE TABLE "banner_feature" (
  "banner_id" bigserial UNIQUE,
  "feature_id" bigserial,
  UNIQUE ("banner_id", "feature_id")
);

CREATE TABLE "tokens" (
  "id" bigserial PRIMARY KEY,
  "token" UNIQUE,
  "is_admin" boolean,
  "created_at" timestamp
);

ALTER TABLE "banner_tag" ADD FOREIGN KEY ("banner_id") REFERENCES "banners" ("id");

ALTER TABLE "banner_tag" ADD FOREIGN KEY ("tag_id") REFERENCES "tags" ("id");

ALTER TABLE "banner_feature" ADD FOREIGN KEY ("banner_id") REFERENCES "banners" ("id");

ALTER TABLE "banner_feature" ADD FOREIGN KEY ("feature_id") REFERENCES "features" ("id");

INSERT INTO
  tags ("id")
VALUES
   (1),(2),(3),(4),(5);

