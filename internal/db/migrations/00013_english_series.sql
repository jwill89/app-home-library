-- +goose Up

-- Open Library named two manga series in Japanese, which split The Ideal Sponger Life's
-- volumes three ways and put a Japanese name before High School Prodigies. Their books join
-- the English-named series; a lookup keeps the English half of such a name.
UPDATE works SET series_id = (SELECT id FROM series WHERE name = 'The Ideal Sponger Life')
WHERE series_id IN (
    SELECT id FROM series
    WHERE name IN ('漫画 理想のヒモ生活', '漫画 理想のヒモ生活 [The Ideal Sponger Life] (Manga)')
)
AND EXISTS (SELECT 1 FROM series WHERE name = 'The Ideal Sponger Life');

DELETE FROM series
WHERE name IN ('漫画 理想のヒモ生活', '漫画 理想のヒモ生活 [The Ideal Sponger Life] (Manga)')
AND NOT EXISTS (SELECT 1 FROM works WHERE works.series_id = series.id);

UPDATE series SET name = 'High School Prodigies Have It Easy Even in Another World! Manga'
WHERE name = '漫画 超人高校生たちは異世界でも餘裕で生き抜くようです！ / High School Prodigies Have It Easy Even in Another World! Manga'
AND NOT EXISTS (SELECT 1 FROM series WHERE name = 'High School Prodigies Have It Easy Even in Another World! Manga');

-- +goose Down
SELECT 1;
