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
    , updated_at real not null
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
        , case when old.dir != new.dir then old.dir else null end
        , case when old.name != new.name then old.name else null end
        , case when old.ext != new.ext then old.ext else null end
        , case when old.mime != new.mime then old.mime else null end
        , case when old.is_dir != new.is_dir then old.is_dir else null end
        , old.updated_at
        , old.v
        -- , 1
        -- max (1 << 5) - 1
        , coalesce(
            ((old.dir != new.dir) << 0)
            | ((old.name != new.name) << 1)
            | ((old.ext != new.ext) << 2)
            -- 6 when both name and ext change
            -- | ((old.mime != new.mime) << 3)
            -- | ((old.is_dir != new.is_dir) << 4)
            , 0
        )
    ;
end;

-- create table journals_history (
--   journal_id text
--   , title text
--   , owned_by text
--   , updated_at real
--   , _version int not null
--   , _mask int not null
--   , primary key (journal_id, _version)
-- ) without rowid;

-- create trigger update_journals_history
-- after update on journals for each row
-- begin
--   insert into journals_history (journal_id, title, owned_by, updated_at, _version, _mask)
--   select old.id
--     , case when old.title != new.title then old.title else null end
--     , case when old.owned_by != new.owned_by then old.owned_by else null end
--     , old.updated_at
--     -- , 0.0
--     , old._version
--     , coalesce(((old.title != new.title) << 0)
--     | ((old.owned_by != new.owned_by) << 1)
--     , 0)
--   ;
-- end;