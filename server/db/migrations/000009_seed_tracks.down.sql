DELETE FROM tracks WHERE code IN (
    'full_paper', 'short_paper', 'workshop', 'doctoral_consortium',
    'demo', 'journal_first', 'poster'
);
