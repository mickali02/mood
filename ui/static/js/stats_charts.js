// ui/static/js/stats_charts.js

// Simple debounce function
function debounce(func, wait, immediate) {
    var timeout;
    return function() {
        var context = this, args = arguments;
        var later = function() {
            timeout = null;
            if (!immediate) func.apply(context, args);
        };
        var callNow = immediate && !timeout;
        clearTimeout(timeout);
        timeout = setTimeout(later, wait);
        if (callNow) func.apply(context, args);
    };
};


// Wait for the DOM to be fully loaded before running chart logic
document.addEventListener('DOMContentLoaded', () => {
    // Get the container element holding the data attributes
    const dataContainer = document.getElementById('stats-data-container');

    // Array to hold our chart instances
    const chartInstances = [];

    // Check if the data container exists
    if (!dataContainer) {
        console.warn("Stats data container not found. Charts cannot be rendered.");
        return;
    }

    // Retrieve data from data attributes
    const emotionDataString = dataContainer.dataset.emotionCounts;
    const monthlyDataString = dataContainer.dataset.monthlyCounts;
    const hasData = dataContainer.dataset.hasData === 'true';

    // Only proceed if we have data and the necessary JSON strings
    if (hasData && emotionDataString && monthlyDataString) {
        try {
            // Parse the JSON data passed from the Go template
            const emotionCounts = JSON.parse(emotionDataString);
            const monthlyCounts = JSON.parse(monthlyDataString);

            // --- Prepare data arrays for charts ---
            const emotionLabels = emotionCounts.map(item => `${item.emoji} ${item.name}`);
            const emotionDataValues = emotionCounts.map(item => item.count);
            const emotionColors = emotionCounts.map(item => item.color); // Use colors from DB

            const monthlyLabels = monthlyCounts.map(item => item.month);
            const monthlyDataValues = monthlyCounts.map(item => item.count);

            // --- Common Chart Options for Responsiveness ---
            const commonChartOptions = {
                responsive: true, // Explicitly true (default in v3+)
                maintainAspectRatio: false, // Crucial for filling container height
                // resizeDelay: 100, // Optional: Add a small delay if needed
            };

            // --- Render Emotion Bar Chart ---
            const ctxBar = document.getElementById('emotionBarChart');
            if (ctxBar) {
                const barChart = new Chart(ctxBar, { // Assign to variable
                    type: 'bar',
                    data: {
                        labels: emotionLabels,
                        datasets: [{
                            label: 'Entries',
                            data: emotionDataValues,
                            backgroundColor: emotionColors,
                            borderColor: emotionColors.map(c => lightenDarkenColor(c, -20)),
                            borderWidth: 1
                        }]
                    },
                    options: {
                        ...commonChartOptions, // Spread common options
                        scales: {
                            y: { beginAtZero: true, ticks: { color: '#e0e0e0' }, grid: { color: 'rgba(255, 255, 255, 0.1)' } },
                            x: { ticks: { color: '#e0e0e0' }, grid: { color: 'rgba(255, 255, 255, 0.1)' } }
                        },
                        plugins: { legend: { display: false } },
                    }
                });
                chartInstances.push(barChart); // Store instance
            } else {
                console.warn("Canvas element for Emotion Bar Chart not found.");
            }

            // --- Render Emotion Pie Chart ---
            const ctxPie = document.getElementById('emotionPieChart');
            if (ctxPie) {
                const pieChart = new Chart(ctxPie, { // Assign to variable
                    type: 'pie',
                    data: {
                        labels: emotionLabels,
                        datasets: [{
                            label: 'Emotion Breakdown',
                            data: emotionDataValues,
                            backgroundColor: emotionColors,
                            hoverOffset: 4
                        }]
                    },
                    options: {
                        ...commonChartOptions, // Spread common options
                        plugins: {
                            legend: {
                                position: 'top',
                                labels: { color: '#e0e0e0' }
                            }
                        }
                    }
                });
                chartInstances.push(pieChart); // Store instance
            } else {
                console.warn("Canvas element for Emotion Pie Chart not found.");
            }

            // --- Render Monthly Line Chart ---
            const ctxLine = document.getElementById('monthlyLineChart');
            if (ctxLine) {
                const lineChart = new Chart(ctxLine, {
                    type: 'line',
                    data: {
                        labels: monthlyLabels,
                        datasets: [{
                            label: 'Entries per Month',
                            data: monthlyDataValues,
                            fill: false,
                            borderColor: '#A074B5',
                            tension: 0.1
                            // Point styling now handled in options.elements.point below
                        }]
                    },
                    options: {
                        ...commonChartOptions, // Use common responsive options
                        scales: {
                            y: {
                                beginAtZero: true,
                                ticks: { color: '#e0e0e0', stepSize: 1 },
                                grid: { color: 'rgba(255, 255, 255, 0.1)' },
                                // *** ADDED Y-AXIS TITLE ***
                                title: {
                                    display: true,
                                    text: 'Number of Entries',
                                    color: '#bdc1c6',
                                    font: { size: 14, family: 'Poppins' }
                                }
                             },
                            x: {
                                ticks: { color: '#e0e0e0' },
                                grid: { color: 'rgba(255, 255, 255, 0.1)' }
                             }
                        },
                        // *** ADDED/ENHANCED POINT STYLING ***
                        elements: {
                            point: {
                                radius: 5, // Size of the dot
                                hoverRadius: 7, // Size on hover
                                backgroundColor: '#e0e0e0', // Fill color
                                borderColor: '#A074B5',     // Border color (matches line)
                                borderWidth: 2          // Border thickness
                            }
                        },
                        plugins: {
                             legend: { display: false },
                             tooltip: { /* Keep existing tooltip config if any */ }
                         }
                    }
                });
                chartInstances.push(lineChart); // Store instance
            } else {
                console.warn("Canvas element for Monthly Line Chart not found.");
            }

        } catch (e) {
            console.error("Error parsing chart data or rendering charts:", e);
        }
    } else if (!hasData) {
        console.info("No mood entries found, charts will not be rendered.");
    } else {
         console.warn("Missing required data for charts (emotion or monthly counts).");
    }

    // --- Resize Handling ---
    const handleResize = () => {
        chartInstances.forEach(chart => {
            if (chart) {
                chart.resize();
            }
        });
    };
    window.addEventListener('resize', debounce(handleResize, 250));

});

// Helper function to adjust color brightness (optional, for borders)
function lightenDarkenColor(col, amt) {
    var usePound = false;
    if (col && col[0] == "#") {
        col = col.slice(1);
        usePound = true;
    } else {
        return col;
    }
    var num = parseInt(col, 16);
    if (isNaN(num)) {
         return (usePound ? "#" : "") + col;
    }
    var r = (num >> 16) + amt;
    if (r > 255) r = 255; else if (r < 0) r = 0;
    var b = ((num >> 8) & 0x00FF) + amt;
    if (b > 255) b = 255; else if (b < 0) b = 0;
    var g = (num & 0x0000FF) + amt;
    if (g > 255) g = 255; else if (g < 0) g = 0;
    var newColor = (r << 16 | b << 8 | g).toString(16);
    while(newColor.length < 6) {
        newColor = "0" + newColor;
    }
    return (usePound ? "#" : "") + newColor;
}