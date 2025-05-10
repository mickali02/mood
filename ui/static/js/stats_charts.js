// mood/ui/static/js/stats_charts.js
document.addEventListener('DOMContentLoaded', function () {
    const statsContainer = document.getElementById('stats-data-container');
    if (!statsContainer) {
        console.error('[Global] Stats container (stats-data-container) not found');
        return;
    }

    // Attempt to register the datalabels plugin
    try {
        if (Chart && Chart.register) {
            Chart.register(ChartDataLabels);
            console.log("[Global] ChartDataLabels registered successfully.");
        } else {
            console.error("[Global] Chart object or Chart.register is not available. Chart.js might not be loaded correctly.");
            // Display a general error message on the page if Chart.js itself is missing
            showNoDataMessage("Error: Charting library failed to load. Statistics cannot be displayed.");
            const mainContent = document.querySelector('.stats-main-content');
            const loadingIndicator = document.querySelector('.stats-loading-indicator');
            if (statsContainer) statsContainer.classList.remove('is-loading');
            if (loadingIndicator) loadingIndicator.style.display = 'none';
            if (mainContent) {
                mainContent.style.opacity = '1';
                mainContent.style.visibility = 'visible';
                mainContent.classList.add('is-loaded'); // Still mark as loaded to show the error message
            }
            return; // Stop further execution if Chart.js is not available
        }
    } catch (pluginError) {
        console.error("[Global] Error registering ChartDataLabels plugin:", pluginError);
        // Proceed without datalabels if registration fails, or handle more gracefully
    }


    const hasData = statsContainer.dataset.hasData === 'true';
    console.log("[Global] statsContainer.dataset.hasData:", statsContainer.dataset.hasData, "(parsed as boolean:", hasData + ")");


    if (hasData) {
        try {
            const emotionCountsJSON = statsContainer.dataset.emotionCounts || '[]';
            console.log("[Global] Raw emotionCountsJSON from data attribute:", emotionCountsJSON);
            const emotionCountsData = JSON.parse(emotionCountsJSON);
            console.log("[Global] Parsed emotionCountsData:", JSON.stringify(emotionCountsData));

            if (emotionCountsData.length > 0) {
                console.log("[Global] Data found (length > 0), calling initializeCharts...");
                initializeCharts(emotionCountsData);
            } else {
                console.warn('[Global] No emotion counts data available for charts (emotionCountsData.length is 0).');
                showNoDataMessage(); // Show general "no data" message for the page
                handlePieChartLoaderState('No data for this chart.', false); // Specifically update pie chart area
            }
        } catch (e) {
            console.error('[Global] Error parsing stats data (emotionCountsJSON):', e);
            showNoDataMessage();
            handlePieChartLoaderState('Error loading chart data.', false);
        }
    } else {
        console.info('[Global] No mood entries (hasData is false), stats will not be displayed.');
        showNoDataMessage();
        handlePieChartLoaderState('No data available for charts.', false);
    }

    // This section manages the overall page loading state, ensuring the main content area becomes visible.
    const mainContent = document.querySelector('.stats-main-content');
    const loadingIndicator = document.querySelector('.stats-loading-indicator'); // Global page loader
    if (statsContainer) statsContainer.classList.remove('is-loading');
    if (loadingIndicator) loadingIndicator.style.display = 'none';
    if (mainContent) {
        mainContent.style.opacity = '1';
        mainContent.style.visibility = 'visible';
        mainContent.classList.add('is-loaded');
        console.log("[Global] Main content area made visible.");
    } else {
        console.warn("[Global] Main content area (.stats-main-content) not found.");
    }
});

// Helper function to manage pie chart loader and canvas visibility
function handlePieChartLoaderState(loaderMessage, showCanvas) {
    const pieChartLoader = document.getElementById('pieChartLoadingPlaceholder');
    const pieChartCanvasElement = document.getElementById('emotionPieChart');

    if (pieChartLoader) {
        // Always hide the loader div itself. The text content was for the animated dots.
        // If there's a message, it means something went wrong or no data, so canvas shouldn't show.
        pieChartLoader.style.display = 'none';
        console.log(`[handlePieChartLoaderState] Pie Loader (pieChartLoadingPlaceholder) display set to 'none'. Original message intended: "${loaderMessage || 'N/A'}"`);
        if (loaderMessage && !showCanvas) { // If there's a message and we are NOT showing the canvas,
                                          // it implies an error or no data for the pie chart.
                                          // We could display this message elsewhere if needed, or let the main page's no-data message suffice.
             console.warn(`[handlePieChartLoaderState] Pie chart specific issue or no data: "${loaderMessage}"`);
        }
    } else {
        console.warn("[handlePieChartLoaderState] Pie chart loader element (pieChartLoadingPlaceholder) not found.");
    }

    if (pieChartCanvasElement) {
        pieChartCanvasElement.style.display = showCanvas ? 'block' : 'none';
        console.log(`[handlePieChartLoaderState] Pie Canvas (emotionPieChart) display set to '${showCanvas ? 'block' : 'none'}'`);
    } else {
        console.warn("[handlePieChartLoaderState] Pie chart canvas element (emotionPieChart) not found.");
    }
}


function initializeCharts(emotionCountsData) {
    const labels = emotionCountsData.map(item => item.name);
    const dataForCharts = emotionCountsData.map(item => item.count);
    const backgroundColors = emotionCountsData.map(item => item.color);
    const emojis = emotionCountsData.map(item => item.emoji);

    console.log("[initializeCharts] Labels:", JSON.stringify(labels));
    console.log("[initializeCharts] Data for charts (counts):", JSON.stringify(dataForCharts));
    console.log("[initializeCharts] BackgroundColors:", JSON.stringify(backgroundColors));
    console.log("[initializeCharts] Emojis:", JSON.stringify(emojis));

    const baseChartOptions = {
        responsive: true,
        maintainAspectRatio: false,
        animation: false, // Disable animations globally for charts by default
        plugins: {
            legend: {
                display: true,
                position: 'bottom',
                labels: {
                    color: '#e0e0e0',
                    font: { size: 11, family: "'Poppins', sans-serif" },
                    padding: 15,
                    usePointStyle: true,
                    boxWidth: 8,
                }
            },
            tooltip: {
                enabled: true,
                backgroundColor: 'rgba(0,0,0,0.75)',
                titleFont: { size: 14, family: "'Playfair Display', serif" },
                bodyFont: { size: 12, family: "'Poppins', sans-serif" },
                padding: 10,
                cornerRadius: 4,
                callbacks: {
                    label: function(context) {
                        let label = context.dataset.label || ''; // E.g., 'Emotion Breakdown' or 'Emotion Count'
                        let value = context.raw; // The raw data value

                        if (context.chart.config.type === 'pie') {
                            // For pie charts, context.label is the item label (e.g., "😊 Happy")
                            return `${context.label}: ${value}`;
                        } else if (context.chart.config.type === 'bar') {
                            // For bar charts, context.label is the x-axis category label
                            return `${context.label}: ${value}`;
                        }
                        return `${label}: ${value}`; // Fallback
                    }
                }
            }
        },
    };

    // --- Bar Chart ---
    const barCtx = document.getElementById('emotionBarChart')?.getContext('2d');
    if (barCtx) {
        console.log("[BarChart] Initializing Bar Chart...");
        try {
            barCtx.canvas.style.height = '100%';
            barCtx.canvas.style.width = '100%';
            new Chart(barCtx, {
                type: 'bar',
                data: {
                    labels: labels.map((label, index) => `${emojis[index]} ${label}`),
                    datasets: [{
                        label: 'Emotion Count', // This will be used in tooltip by default if not overridden
                        data: dataForCharts,
                        backgroundColor: backgroundColors,
                        borderColor: backgroundColors.map(color => chroma(color).darken(0.5).hex()),
                        borderWidth: 1,
                        borderRadius: 4,
                        barPercentage: 0.7,
                        categoryPercentage: 0.8
                    }]
                },
                options: {
                    ...baseChartOptions,
                    scales: {
                        y: {
                            beginAtZero: true,
                            ticks: { color: '#bdc1c6', font: { size: 11, family: "'Poppins', sans-serif" }, stepSize: 1, precision: 0 },
                            grid: { color: 'rgba(255, 255, 255, 0.1)', borderColor: 'rgba(255, 255, 255, 0.1)' }
                        },
                        x: {
                            ticks: { color: '#bdc1c6', font: { size: 10, family: "'Poppins', sans-serif" }, maxRotation: 45, minRotation: 0 },
                            grid: { display: false }
                        }
                    },
                    plugins: {
                        ...baseChartOptions.plugins, // Inherit base plugins
                        legend: { ...baseChartOptions.plugins.legend, display: false }, // Explicitly disable legend for bar
                        datalabels: { display: false } // Explicitly disable datalabels for bar
                    }
                }
            });
            console.log("[BarChart] Bar Chart Initialized Successfully.");
        } catch (barError) {
            console.error("[BarChart] ERROR Initializing Bar Chart:", barError);
        }
    } else {
        console.warn('[BarChart] Bar chart canvas (emotionBarChart) not found.');
    }

    // --- Pie Chart ---
    console.log("[PieChart] Preparing to initialize Pie Chart...");
    const pieChartCanvasElement = document.getElementById('emotionPieChart');

    if (pieChartCanvasElement) {
        const pieCtx = pieChartCanvasElement.getContext('2d');
        if (pieCtx) {
            const sumOfData = dataForCharts.reduce((a, b) => a + b, 0);
            console.log("[PieChart] Sum of dataForCharts for Pie Chart:", sumOfData);
            if (sumOfData === 0 && dataForCharts.length > 0) {
                console.warn("[PieChart] Data for pie chart sums to zero. The chart might appear empty or only show legend.");
                // Even if sum is zero, we proceed to initialize it, Chart.js should handle it gracefully (empty pie)
            }

            try {
                console.log("[PieChart] Setting canvas style and attempting new Chart() for Pie Chart...");
                pieChartCanvasElement.style.height = '100%';
                pieChartCanvasElement.style.width = '100%';
                
                new Chart(pieCtx, {
                    type: 'pie',
                    data: {
                        labels: labels.map((label, index) => `${emojis[index]} ${label}`),
                        datasets: [{
                            label: 'Emotion Breakdown', // Used in tooltip
                            data: dataForCharts,
                            backgroundColor: backgroundColors,
                            borderColor: backgroundColors.map(color => chroma(color).darken(0.7).hex()),
                            borderWidth: 2,
                            hoverOffset: 10
                        }]
                    },
                    options: {
                        ...baseChartOptions, // Inherits animation: false
                        plugins: {
                            ...baseChartOptions.plugins, // Inherit base plugins
                            legend: { ...baseChartOptions.plugins.legend, display: true }, // Ensure legend is ON for pie
                            datalabels: {
                                display: true,
                                formatter: (value, ctx) => {
                                    const datapoints = ctx.chart.data.datasets[0].data;
                                    const total = datapoints.reduce((totalVal, datapoint) => totalVal + datapoint, 0);
                                    if (total === 0 || value === 0) return ''; // Don't show label for 0 value or if total is 0
                                    const percentage = (value / total) * 100;
                                    // Only show percentage if it's somewhat significant
                                    return percentage >= 1 ? percentage.toFixed(1) + '%' : '';
                                },
                                color: '#ffffff',
                                font: { weight: 'bold', size: 11, family: "'Poppins', sans-serif" },
                                anchor: 'center',
                                align: 'center'
                            }
                        }
                    }
                });
                console.log("[PieChart] Pie Chart new Chart() call COMPLETED.");
                handlePieChartLoaderState(null, true); // null message means hide loader, true means show canvas

            } catch (pieError) {
                console.error('[PieChart] ERROR initializing Pie Chart:', pieError);
                handlePieChartLoaderState('Error rendering chart.', false); // Show error message, hide canvas
            }
        } else {
            console.warn('[PieChart] Could not get 2D context for pie chart canvas (emotionPieChart).');
            handlePieChartLoaderState('Chart context error.', false);
        }
    } else {
        console.warn('[PieChart] Pie chart canvas element (emotionPieChart) not found.');
        handlePieChartLoaderState('Chart display area not found.', false); // If canvas isn't there, loader doesn't matter
    }
}

// This function is called if hasData is false OR if emotionCountsData is empty OR if JSON parsing fails
function showNoDataMessage(customMessage = "") {
    const mainContent = document.querySelector('.stats-main-content');
    if (mainContent) {
        console.log("[showNoDataMessage] Displaying no data message.");
        // Check if a no-stats div is already present (e.g., from server-side template for 0 total entries)
        const existingNoStatsDiv = mainContent.querySelector('.no-stats');
        if (existingNoStatsDiv) {
            console.log("[showNoDataMessage] Found existing .no-stats div, will not overwrite.");
            // Optionally update its text if a custom message is provided
            if (customMessage && existingNoStatsDiv.querySelector('p')) {
                existingNoStatsDiv.querySelector('p').textContent = customMessage;
            }
        } else {
            const message = customMessage || "You haven't logged enough moods to generate statistics yet. Keep tracking!";
            mainContent.innerHTML = `
                <div class="no-stats">
                    <p>${message}</p>
                    <a href="/dashboard" class="back-link">← Back to Dashboard</a>
                </div>`;
        }
    } else {
        console.warn("[showNoDataMessage] .stats-main-content area not found to display no data message.");
    }
}