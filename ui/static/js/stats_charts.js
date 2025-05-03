// ui/static/js/stats_charts.js

// Wait for the DOM to be fully loaded before running chart logic
document.addEventListener('DOMContentLoaded', () => {
    // Get the container element holding the data attributes
    const dataContainer = document.getElementById('stats-data-container');

    // Check if the data container exists
    if (!dataContainer) {
        console.warn("Stats data container not found. Charts cannot be rendered.");
        return;
    }

    // Retrieve data from data attributes
    const emotionDataString = dataContainer.dataset.emotionCounts;
    const monthlyDataString = dataContainer.dataset.monthlyCounts;
    // dataset values are strings, so compare with 'true'
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

            // --- Render Emotion Bar Chart ---
            const ctxBar = document.getElementById('emotionBarChart');
            if (ctxBar) {
                new Chart(ctxBar, {
                    type: 'bar',
                    data: {
                        labels: emotionLabels,
                        datasets: [{
                            label: 'Entries',
                            data: emotionDataValues,
                            backgroundColor: emotionColors, // Use colors for bars
                            borderColor: emotionColors.map(c => lightenDarkenColor(c, -20)), // Slightly darker border
                            borderWidth: 1
                        }]
                    },
                    options: {
                        scales: {
                            y: { beginAtZero: true, ticks: { color: '#e0e0e0' }, grid: { color: 'rgba(255, 255, 255, 0.1)' } },
                            x: { ticks: { color: '#e0e0e0' }, grid: { color: 'rgba(255, 255, 255, 0.1)' } }
                        },
                        plugins: { legend: { display: false } }, // Hide legend for bar chart
                        maintainAspectRatio: false // Allow chart to resize vertically
                    }
                });
            } else {
                console.warn("Canvas element for Emotion Bar Chart not found.");
            }

            // --- Render Emotion Pie Chart ---
            const ctxPie = document.getElementById('emotionPieChart');
            if (ctxPie) {
                new Chart(ctxPie, {
                    type: 'pie',
                    data: {
                        labels: emotionLabels,
                        datasets: [{
                            label: 'Emotion Breakdown',
                            data: emotionDataValues,
                            backgroundColor: emotionColors, // Use colors for slices
                            hoverOffset: 4
                        }]
                    },
                    options: {
                        plugins: {
                            legend: {
                                position: 'top',
                                labels: { color: '#e0e0e0' }
                            }
                        },
                        maintainAspectRatio: false
                    }
                });
            } else {
                console.warn("Canvas element for Emotion Pie Chart not found.");
            }

            // --- Render Monthly Line Chart ---
            const ctxLine = document.getElementById('monthlyLineChart');
            if (ctxLine) {
                new Chart(ctxLine, {
                    type: 'line', // or 'bar' if you prefer
                    data: {
                        labels: monthlyLabels,
                        datasets: [{
                            label: 'Entries per Month',
                            data: monthlyDataValues,
                            fill: false,
                            borderColor: '#A074B5', // Theme color
                            tension: 0.1,
                            pointBackgroundColor: '#e0e0e0',
                            pointBorderColor: '#A074B5'
                        }]
                    },
                    options: {
                        scales: {
                            y: { beginAtZero: true, ticks: { color: '#e0e0e0', stepSize: 1 }, grid: { color: 'rgba(255, 255, 255, 0.1)' } }, // Ensure integer ticks
                            x: { ticks: { color: '#e0e0e0' }, grid: { color: 'rgba(255, 255, 255, 0.1)' } }
                        },
                        plugins: { legend: { display: false } },
                        maintainAspectRatio: false
                    }
                });
            } else {
                console.warn("Canvas element for Monthly Line Chart not found.");
            }

        } catch (e) {
            console.error("Error parsing chart data or rendering charts:", e);
            // Optionally display a user-friendly error message on the page
            // For example: document.getElementById('chart-error-message').style.display = 'block';
        }
    } else if (!hasData) {
        console.info("No mood entries found, charts will not be rendered.");
    } else {
         console.warn("Missing required data for charts (emotion or monthly counts).");
    }
});

// Helper function to adjust color brightness (optional, for borders)
// Keep this function here or move it to a more general utility JS file if you have one
function lightenDarkenColor(col, amt) {
    var usePound = false;
    if (col && col[0] == "#") { // Add null check for col
        col = col.slice(1);
        usePound = true;
    } else {
        // Handle cases where color might not be a valid hex string
        console.warn("Invalid color format passed to lightenDarkenColor:", col);
        return col; // Return original or a default color
    }

    var num = parseInt(col, 16);
    // Check if parsing failed
    if (isNaN(num)) {
         console.warn("Failed to parse color:", col);
         return (usePound ? "#" : "") + col; // Return original
    }

    var r = (num >> 16) + amt;
    if (r > 255) r = 255; else if (r < 0) r = 0;
    var b = ((num >> 8) & 0x00FF) + amt;
    if (b > 255) b = 255; else if (b < 0) b = 0;
    var g = (num & 0x0000FF) + amt;
    if (g > 255) g = 255; else if (g < 0) g = 0;

    // Ensure the resulting hex string is always 6 digits
    var newColor = (r << 16 | b << 8 | g).toString(16);
    while(newColor.length < 6) {
        newColor = "0" + newColor;
    }

    return (usePound ? "#" : "") + newColor;
}