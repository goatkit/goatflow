/**
 * Ticket Attribute Relations - Real-time dropdown filtering
 *
 * This module handles automatic filtering of form dropdowns based on
 * ticket attribute relations defined in the admin panel.
 *
 * Usage:
 * 1. Include this script in your page
 * 2. Initialize with: TicketAttributeRelations.init(options)
 *
 * Options:
 * - apiBase: Base URL for API calls (default: '/api/v1')
 * - debug: Enable debug logging (default: false)
 * - selectors: Object mapping attribute names to CSS selectors
 */

(function(window) {
    'use strict';

    var TicketAttributeRelations = {
        config: {
            apiBase: '/api/v1',
            debug: false,
            // Default selector mappings for common ticket form fields
            selectors: {
                'Queue': '#queue_id, [name="queue_id"]',
                'State': '#next_state, #state_id, [name="state_id"], [name="next_state"]',
                'Priority': '#priority, #priority_id, [name="priority"], [name="priority_id"]',
                'Type': '#type_id, [name="type_id"]',
                'Service': '#service_id, [name="service_id"]',
                'SLA': '#sla_id, [name="sla_id"]'
            },
            // API endpoints for each attribute type
            apiEndpoints: {
                'Queue': '/queues',
                'State': '/states',
                'Priority': '/priorities',
                'Type': '/types',
                'Service': '/services',
                'SLA': '/slas'
            }
        },

        // Store original options for each field
        originalOptions: {},

        // Store current allowed values from relations
        currentAllowedValues: {},

        // Cache for API responses to avoid redundant calls
        cache: {},

        /**
         * Initialize the ticket attribute relations filtering
         */
        init: function(options) {
            var self = this;

            // Merge user options with defaults
            if (options) {
                if (options.apiBase) this.config.apiBase = options.apiBase;
                if (options.debug) this.config.debug = options.debug;
                if (options.selectors) {
                    Object.keys(options.selectors).forEach(function(key) {
                        self.config.selectors[key] = options.selectors[key];
                    });
                }
            }

            this.log('Initializing ticket attribute relations filtering');

            // Store original options for all dropdowns
            this.storeOriginalOptions();

            // Set up change listeners for all source fields
            this.setupChangeListeners();

            this.log('Initialization complete');
        },

        /**
         * Store original options for all dropdowns
         */
        storeOriginalOptions: function() {
            var self = this;

            Object.keys(this.config.selectors).forEach(function(attr) {
                var selector = self.config.selectors[attr];
                var element = document.querySelector(selector);

                if (element && element.tagName === 'SELECT') {
                    self.originalOptions[attr] = [];
                    Array.from(element.options).forEach(function(opt) {
                        self.originalOptions[attr].push({
                            value: opt.value,
                            text: opt.text,
                            selected: opt.selected,
                            dataset: Object.assign({}, opt.dataset)
                        });
                    });
                    self.log('Stored ' + self.originalOptions[attr].length + ' options for ' + attr);
                }
            });
        },

        /**
         * Set up change listeners for all source fields
         */
        setupChangeListeners: function() {
            var self = this;

            Object.keys(this.config.selectors).forEach(function(attr) {
                var selector = self.config.selectors[attr];
                var element = document.querySelector(selector);

                if (element) {
                    element.addEventListener('change', function() {
                        self.handleFieldChange(attr, this.value, this);
                    });
                    self.log('Added change listener for ' + attr);
                }
            });
        },

        /**
         * Handle field change - evaluate relations and filter other dropdowns
         */
        handleFieldChange: function(attribute, value, element) {
            var self = this;

            this.log('Field changed: ' + attribute + ' = ' + value);

            if (!value) {
                // If value is cleared, restore all original options
                this.restoreAllOptions();
                return;
            }

            // Get the display name for the value (for relations that use names not IDs)
            var displayValue = value;
            if (element && element.tagName === 'SELECT') {
                var selectedOption = element.options[element.selectedIndex];
                if (selectedOption) {
                    // Try to get the name from text or data attribute
                    displayValue = selectedOption.dataset.name || selectedOption.text;
                }
            }

            // Call the evaluate endpoint
            this.evaluateRelations(attribute, displayValue, function(result) {
                if (result && result.success && result.allowed_values) {
                    self.currentAllowedValues = result.allowed_values;
                    self.applyFiltering(result.allowed_values);
                }
            });
        },

        /**
         * Call the evaluate API endpoint
         */
        evaluateRelations: function(attribute, value, callback) {
            var self = this;
            var url = this.config.apiBase + '/ticket-attribute-relations/evaluate?attribute=' +
                      encodeURIComponent(attribute) + '&value=' + encodeURIComponent(value);

            this.log('Calling evaluate API: ' + url);

            fetch(url)
                .then(function(response) {
                    return response.json();
                })
                .then(function(data) {
                    self.log('Evaluate response:', data);
                    callback(data);
                })
                .catch(function(error) {
                    self.log('Evaluate error:', error);
                    callback(null);
                });
        },

        /**
         * Apply filtering to dropdowns based on allowed values
         */
        applyFiltering: function(allowedValues) {
            var self = this;

            Object.keys(allowedValues).forEach(function(targetAttr) {
                var allowed = allowedValues[targetAttr];

                if (!allowed || !allowed.length) {
                    return;
                }

                // Check if this is a dynamic field
                if (targetAttr.indexOf('DynamicField_') === 0) {
                    self.filterDynamicField(targetAttr, allowed);
                } else {
                    self.filterStandardField(targetAttr, allowed);
                }
            });
        },

        /**
         * Filter a standard ticket field dropdown
         */
        filterStandardField: function(attr, allowedValues) {
            var self = this;
            var selector = this.config.selectors[attr];

            if (!selector) {
                this.log('No selector configured for ' + attr);
                return;
            }

            var element = document.querySelector(selector);
            if (!element || element.tagName !== 'SELECT') {
                this.log('No select element found for ' + attr);
                return;
            }

            // Create a set for fast lookup
            var allowedSet = {};
            allowedValues.forEach(function(v) {
                allowedSet[v] = true;
            });

            // Get original options
            var original = this.originalOptions[attr] || [];

            // Store current value to try to preserve selection
            var currentValue = element.value;
            var hasCurrentValue = false;

            // Clear and rebuild options
            element.innerHTML = '';

            original.forEach(function(opt) {
                // Always include empty/placeholder options
                if (!opt.value) {
                    var newOpt = document.createElement('option');
                    newOpt.value = opt.value;
                    newOpt.text = opt.text;
                    element.appendChild(newOpt);
                    return;
                }

                // Check if this option is allowed (by text/name)
                if (allowedSet[opt.text]) {
                    var newOpt = document.createElement('option');
                    newOpt.value = opt.value;
                    newOpt.text = opt.text;
                    // Restore dataset attributes
                    Object.keys(opt.dataset || {}).forEach(function(key) {
                        newOpt.dataset[key] = opt.dataset[key];
                    });
                    element.appendChild(newOpt);

                    if (opt.value === currentValue) {
                        hasCurrentValue = true;
                    }
                }
            });

            // Restore selection if still valid, otherwise select first option
            if (hasCurrentValue) {
                element.value = currentValue;
            } else if (element.options.length > 0) {
                element.selectedIndex = 0;
            }

            this.log('Filtered ' + attr + ' to ' + (element.options.length - 1) + ' options');

            // Dispatch change event so other listeners can react
            element.dispatchEvent(new Event('change', { bubbles: true }));
        },

        /**
         * Filter a dynamic field dropdown
         */
        filterDynamicField: function(fieldName, allowedValues) {
            // Extract field name from "DynamicField_FieldName"
            var dfName = fieldName.replace('DynamicField_', '');

            // Try common selector patterns for dynamic fields
            var selectors = [
                '#DynamicField_' + dfName,
                '[name="DynamicField_' + dfName + '"]',
                '[name="DynamicField_' + dfName + '[]"]',
                '#df_' + dfName,
                '[data-dynamic-field="' + dfName + '"]'
            ];

            var element = null;
            for (var i = 0; i < selectors.length; i++) {
                element = document.querySelector(selectors[i]);
                if (element) break;
            }

            if (!element || (element.tagName !== 'SELECT')) {
                this.log('No select element found for dynamic field ' + dfName);
                return;
            }

            // Create a set for fast lookup
            var allowedSet = {};
            allowedValues.forEach(function(v) {
                allowedSet[v] = true;
            });

            // Store original options if not already stored
            if (!this.originalOptions[fieldName]) {
                this.originalOptions[fieldName] = [];
                Array.from(element.options).forEach(function(opt) {
                    this.originalOptions[fieldName].push({
                        value: opt.value,
                        text: opt.text,
                        selected: opt.selected
                    });
                }.bind(this));
            }

            var original = this.originalOptions[fieldName] || [];
            var currentValue = element.value;
            var hasCurrentValue = false;

            // Clear and rebuild options
            element.innerHTML = '';

            original.forEach(function(opt) {
                // Always include empty/placeholder options
                if (!opt.value) {
                    var newOpt = document.createElement('option');
                    newOpt.value = opt.value;
                    newOpt.text = opt.text;
                    element.appendChild(newOpt);
                    return;
                }

                // Check if this option is allowed (by value or text)
                if (allowedSet[opt.value] || allowedSet[opt.text]) {
                    var newOpt = document.createElement('option');
                    newOpt.value = opt.value;
                    newOpt.text = opt.text;
                    element.appendChild(newOpt);

                    if (opt.value === currentValue) {
                        hasCurrentValue = true;
                    }
                }
            });

            // Restore selection if still valid
            if (hasCurrentValue) {
                element.value = currentValue;
            } else if (element.options.length > 0) {
                element.selectedIndex = 0;
            }

            this.log('Filtered DynamicField ' + dfName + ' to ' + (element.options.length - 1) + ' options');
        },

        /**
         * Restore all fields to their original options
         */
        restoreAllOptions: function() {
            var self = this;

            Object.keys(this.originalOptions).forEach(function(attr) {
                self.restoreOptions(attr);
            });

            this.currentAllowedValues = {};
            this.log('Restored all options to original state');
        },

        /**
         * Restore a single field to its original options
         */
        restoreOptions: function(attr) {
            var selector = this.config.selectors[attr];

            // Handle dynamic fields
            if (attr.indexOf('DynamicField_') === 0) {
                var dfName = attr.replace('DynamicField_', '');
                selector = '#DynamicField_' + dfName + ', [name="DynamicField_' + dfName + '"]';
            }

            if (!selector) return;

            var element = document.querySelector(selector);
            if (!element || element.tagName !== 'SELECT') return;

            var original = this.originalOptions[attr] || [];
            if (!original.length) return;

            var currentValue = element.value;
            element.innerHTML = '';

            original.forEach(function(opt) {
                var newOpt = document.createElement('option');
                newOpt.value = opt.value;
                newOpt.text = opt.text;
                // Restore dataset attributes
                Object.keys(opt.dataset || {}).forEach(function(key) {
                    newOpt.dataset[key] = opt.dataset[key];
                });
                element.appendChild(newOpt);
            });

            // Try to restore selection
            element.value = currentValue;
            if (element.selectedIndex === -1 && element.options.length > 0) {
                element.selectedIndex = 0;
            }
        },

        /**
         * Manually trigger evaluation (useful after programmatic changes)
         */
        evaluate: function(attribute, value) {
            var self = this;
            var selector = this.config.selectors[attribute];
            var element = document.querySelector(selector);

            this.handleFieldChange(attribute, value, element);
        },

        /**
         * Debug logging
         */
        log: function() {
            if (this.config.debug) {
                console.log.apply(console, ['[TicketAttributeRelations]'].concat(Array.from(arguments)));
            }
        }
    };

    // Expose to global scope
    window.TicketAttributeRelations = TicketAttributeRelations;

})(window);
