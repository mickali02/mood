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

// Helper function to convert hex color to rgba (for transparency)
const hexToRgba = (hex, alpha = 0.7) => {
    let r = 0, g = 0, b = 0;
    if (!hex) return `rgba(204, 204, 204, ${alpha})`; // Default gray if hex is missing

    hex = hex.replace('#', ''); // Remove # if present

    // 3 digits
    if (hex.length === 3) {
        r = parseInt(hex[0] + hex[0], 16);
        g = parseInt(hex[1] + hex[1], 16);
        b = parseInt(hex[2] + hex[2], 16);
    }
    // 6 digits
    else if (hex.length === 6) {
        r = parseInt(hex[0] + hex[1], 16);
        g = parseInt(hex[2] + hex[3], 16);
        b = parseInt(hex[4] + hex[5], 16);
    }
    return `rgba(${r}, ${g}, ${b}, ${alpha})`;
};


// Wait for the DOM to be fully loaded before running chart logic
document.addEventListener('DOMContentLoaded', () => {
    const dataContainer = document.getElementById('stats-data-container');
    const chartInstances = [];

    if (!dataContainer) {
        console.warn("Stats data container not found. Charts cannot be rendered.");
        return;
    }

    const emotionDataString = dataContainer.dataset.emotionCounts;
    const monthlyDataString = dataContainer.dataset.monthlyCounts;
    const hasData = dataContainer.dataset.hasData === 'true';

    if (hasData && emotionDataString && monthlyDataString) {
        try {
            if (typeof ChartDataLabels !== 'undefined') {
                Chart.register(ChartDataLabels);
                console.log("ChartDataLabels plugin registered.");
            } else {
                console.warn("ChartDataLabels plugin not found. Make sure it's included in the HTML.");
            }

            const emotionCounts = JSON.parse(emotionDataString);
            const monthlyCounts = JSON.parse(monthlyDataString);

            const emotionLabelsRaw = emotionCounts.map(item => `${item.emoji} ${item.name}`);
            const emotionDataValues = emotionCounts.map(item => item.count);
            const emotionColors = emotionCounts.map(item => item.color || '#cccccc');

            const monthlyLabels = monthlyCounts.map(item => item.month);
            const monthlyDataValues = monthlyCounts.map(item => item.count);

            const commonChartOptions = {
                responsive: true,
                maintainAspectRatio: false,
            };

            // --- Render Emotion Bar Chart (WITH Y-Axis Title) ---
            const ctxBar = document.getElementById('emotionBarChart');
            if (ctxBar && emotionCounts.length > 0) {
                const barChart = new Chart(ctxBar, {
                    type: 'bar',
                    data: {
                        labels: emotionLabelsRaw,
                        datasets: [{
                            label: 'Entries Count',
                            data: emotionDataValues,
                            backgroundColor: emotionColors.map(c => hexToRgba(c, 0.7)),
                            borderColor: emotionColors,
                            borderWidth: 1
                        }]
                    },
                    options: {
                        ...commonChartOptions,
                        scales: {
                            y: { // Y-AXIS Configuration
                                beginAtZero: true,
                                ticks: { color: '#e0e0e0', precision: 0 },
                                grid: { color: 'rgba(255, 255, 255, 0.1)' },
                                // --- ADDED Y-AXIS TITLE ---
                                title: {
                                    display: true,
                                    text: 'Number of Entries', // Your desired Y-axis label
                                    color: '#bdc1c6',
                                    font: { size: 14, family: "'Poppins', sans-serif" }
                                }
                                // --- END Y-AXIS TITLE ---
                            },
                            x: { // X-AXIS Configuration
                                ticks: { color: '#e0e0e0' },
                                grid: { display: false },
                                title: { // Keep X-axis title
                                    display: true,
                                    text: 'Emotion',
                                    color: '#bdc1c6',
                                    font: { size: 14, family: "'Poppins', sans-serif" }
                                }
                            }
                        },
                        plugins: {
                            legend: { display: false },
                            tooltip: {
                                 bodyFont: { family: "'Poppins', sans-serif" },
                                 titleFont: { family: "'Poppins', sans-serif", weight: 'bold' }
                            },
                            datalabels: { display: false } // Keep hidden on bars
                        },
                    }
                });
                chartInstances.push(barChart);
            } else if (ctxBar) {
                console.warn("Canvas element for Emotion Bar Chart found, but no data.");
            } else {
                 console.warn("Canvas element for Emotion Bar Chart not found.");
            }

            // --- Render Emotion Pie Chart (Percentages ON Slices, RAW Labels in Legend, NO text outline) ---
            const ctxPie = document.getElementById('emotionPieChart');
            if (ctxPie && emotionCounts.length > 0) {
                const totalCount = emotionDataValues.reduce((sum, value) => sum + value, 0);

                const pieChart = new Chart(ctxPie, {
                    type: 'pie',
                    data: {
                        // --- CHANGED: Use RAW labels for legend ---
                        labels: emotionLabelsRaw,
                        // --- END CHANGE ---
                        datasets: [{
                            label: 'Emotion Count',
                            data: emotionDataValues,
                            backgroundColor: emotionColors,
                            borderColor: emotionColors.map(color => hexToRgba(color, 1.0)),
                            borderWidth: 1,
                            hoverOffset: 4
                        }]
                    },
                    options: {
                        ...commonChartOptions,
                        plugins: {
                            legend: { // Legend now shows raw labels
                                position: 'bottom',
                                labels: {
                                    color: '#e0e0e0',
                                    padding: 15,
                                    font: { family: "'Poppins', sans-serif", size: 13 }
                                 }
                            },
                            tooltip: { // Tooltip still shows percentages
                                callbacks: {
                                    label: function(tooltipItem) {
                                        const value = tooltipItem.raw;
                                        const label = emotionLabelsRaw[tooltipItem.dataIndex] || '';
                                        const percentage = totalCount > 0 ? ((value / totalCount) * 100).toFixed(1) : 0;
                                        return `${label}: ${value} (${percentage}%)`;
                                    }
                                },
                                bodyFont: { family: "'Poppins', sans-serif" },
                                titleFont: { family: "'Poppins', sans-serif", weight: 'bold' }
                            },
                             datalabels: { // Configure labels on slices
                                 display: true,
                                 formatter: (value, ctx) => {
                                     const currentTotal = ctx.chart.data.datasets[0].data.reduce((a, b) => a + b, 0);
                                     const percentage = currentTotal > 0 ? ((value / currentTotal) * 100).toFixed(1) : 0;
                                     if (percentage < 3) { // Hide small labels
                                         return '';
                                     }
                                     return `${percentage}%`;
                                 },
                                 color: '#ffffff', // White text
                                 font: {
                                     weight: 'bold',
                                     family: "'Poppins', sans-serif",
                                     size: 12
                                 },
                                 // --- REMOVED textStrokeColor and textStrokeWidth ---
                             }
                        }
                    }
                });
                chartInstances.push(pieChart);
            } else if (ctxPie) {
                console.warn("Canvas element for Emotion Pie Chart found, but no data.");
            } else {
                 console.warn("Canvas element for Emotion Pie Chart not found.");
            }

            // --- Render Monthly Line Chart (NO Data Labels) ---
            const ctxLine = document.getElementById('monthlyLineChart');
            if (ctxLine && monthlyCounts.length > 0) {
                const lineChart = new Chart(ctxLine, {
                    type: 'line',
                    data: {
                        labels: monthlyLabels,
                        datasets: [{
                            label: 'Entries per Month',
                            data: monthlyDataValues,
                            fill: true,
                            backgroundColor: 'rgba(160, 116, 181, 0.2)',
                            borderColor: '#A074B5',
                            tension: 0.3
                        }]
                    },
                    options: {
                        ...commonChartOptions,
                        scales: {
                            y: {
                                beginAtZero: true,
                                ticks: { color: '#e0e0e0', precision: 0 },
                                grid: { color: 'rgba(255, 255, 255, 0.1)' },
                                title: {
                                    display: true,
                                    text: 'Number of Entries',
                                    color: '#bdc1c6',
                                    font: { size: 14, family: "'Poppins', sans-serif" }
                                }
                             },
                            x: {
                                ticks: { color: '#e0e0e0' },
                                grid: { color: 'rgba(255, 255, 255, 0.05)' }
                             }
                        },
                        elements: {
                            point: {
                                radius: 4,
                                hoverRadius: 7,
                                backgroundColor: '#E6D29E',
                                borderColor: '#ffffff',
                                borderWidth: 1,
                                pointHoverBackgroundColor: '#ffffff',
                                pointHoverBorderColor: '#A074B5',
                            }
                        },
                        plugins: {
                             legend: {
                                display: true,
                                position: 'top',
                                labels: { color: '#e0e0e0' }
                             },
                             tooltip: {
                                 bodyFont: { family: "'Poppins', sans-serif" },
                                 titleFont: { family: "'Poppins', sans-serif", weight: 'bold' }
                            },
                             datalabels: { display: false } // Keep hidden on points
                         }
                    }
                });
                chartInstances.push(lineChart);
            } else if (ctxLine) {
                console.warn("Canvas element for Monthly Line Chart found, but no data.");
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
                 // chart.resize(); // May not be needed
            }
        });
    };
    window.addEventListener('resize', debounce(handleResize, 250));

});


// Helper function to adjust color brightness (Unchanged)
function lightenDarkenColor(col, amt) {
    var usePound = false;
    if (col && col[0] == "#") {
        col = col.slice(1);
        usePound = true;
    } else {
        return col || '#cccccc';
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

    r = Math.round(r);
    g = Math.round(g);
    b = Math.round(b);

    var newColor = (r << 16 | g << 8 | b).toString(16);
    while(newColor.length < 6) {
        newColor = "0" + newColor;
    }
    return (usePound ? "#" : "") + newColor;
}