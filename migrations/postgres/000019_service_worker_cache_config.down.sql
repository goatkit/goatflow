DELETE FROM sysconfig_default
 WHERE name IN (
    'ServiceWorker::Enabled',
    'ServiceWorker::DefaultNavigationStrategy',
    'ServiceWorker::Routes'
 );
