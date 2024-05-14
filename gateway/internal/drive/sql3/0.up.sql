create table files (
    -- uuid v7
    id text
    -- uuid v7
    , dir text
    , name text
    , ext text
    , mime text
    , is_dir int as (case when mime is null then 1 else 0 end)
    -- julian time
    , updated_at real not null
    , v int not null
    , check (
        ((mime is null and ext is null and name is not null) or
        (mime is not null and (ext is not null or name is not null))) and 
        (v >= 0)
    )
    , foreign key (dir) references files (id) -- on delete cascade
    , primary key (id)
) strict;

create table files_at (
    id text not null
    , dir text
    , name text
    , ext text
    , mime text
    , is_dir int
    , updated_at real
    , v int not null
    , mask int not null
    , foreign key (id) references files (id)
    , primary key (id, v)
) without rowid;

create trigger update_files_at
after update on files for each row
begin
    insert into files_at (id, dir, name, ext, mime, is_dir, updated_at, v, mask)
    select
        old.id
        -- , case when coalesce(old.dir, '') != coalesce(new.dir, '') then old.dir else null end
        , case when (old.dir != new.dir) then old.dir else null end
        -- , case when coalesce(old.name, '') != coalesce(new.name, '') then old.name else null end
        , case when (coalesce(old.name, '') != coalesce(new.name, '')) then old.name else null end
        -- , case when coalesce(old.ext, '') != coalesce(new.ext, '') then old.ext else null end
        , case when (coalesce(old.ext, '') != coalesce(new.ext, '')) then old.ext else null end
        -- , case when coalesce(old.mime, '') != coalesce(new.mime, '') then old.mime else null end
        , case when (old.mime != new.mime) then old.mime else null end
        -- , case when coalesce(old.is_dir, '') != coalesce(new.is_dir, '') then old.is_dir else null end
        , case when (old.is_dir != new.is_dir) then old.is_dir else null end
        -- , case when coalesce(old.updated_at, 0) != coalesce(new.updated_at, 0) then old.updated_at else null end
        , case when (old.updated_at != new.updated_at) then old.updated_at else null end
        , old.v
        , ((coalesce(old.dir, '') != coalesce(new.dir, '')) << 0)
        | ((coalesce(old.name, '') != coalesce(new.name, '')) << 1)
        | ((coalesce(old.ext, '') != coalesce(new.ext, '')) << 2)
        | ((coalesce(old.mime, '') != coalesce(new.mime, '')) << 3)
        | ((coalesce(old.is_dir, '') != coalesce(new.is_dir, '')) << 4)
        | ((coalesce(old.updated_at, 0) != coalesce(new.updated_at, 0)) << 5)
    ;
end;

-- 0<<0=0
-- 1<<0=1
-- 0<<1=0
-- 1<<1=2
-- 0<<2=0
-- 1<<2=4
-- 0<<3=0
-- 1<<3=8