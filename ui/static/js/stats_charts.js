// mood/ui/static/js/stats_charts.js

document.addEventListener('DOMContentLoaded', () => {
    console.log('DOM fully loaded and parsed');

    const statsDataContainer = document.getElementById('stats-data-container');
    if (!statsDataContainer) {
        console.error('CRITICAL: Stats data container (id="stats-data-container") not found.');
        return;
    }
    console.log('Found statsDataContainer:', statsDataContainer);

    const overallStatsContainer = document.querySelector('.stats-container');
    const mainContent = document.querySelector('.stats-main-content');
    const loadingIndicator = document.querySelector('.stats-loading-indicator');

    if (!overallStatsContainer) console.error('CRITICAL: overallStatsContainer (.stats-container) not found');
    if (!mainContent) console.error('CRITICAL: mainContent (.stats-main-content) not found');
    if (!loadingIndicator) console.warn('Loading indicator (.stats-loading-indicator) not found, but not critical.');

    const hasData = statsDataContainer.dataset.hasData === 'true';
    console.log('hasData:', hasData, '(Raw value: "' + statsDataContainer.dataset.hasData + '")');

    // --- Chart.js Global Defaults (REVISED) ---
    Chart.defaults.animation.duration = 800;
    Chart.defaults.animation.easing = 'easeInOutQuart';
    Chart.defaults.responsive = true;
    // Chart.defaults.maintainAspectRatio = false; // REMOVED: This was causing issues with infinite growth

    // --- Register Datalabels plugin ---
    if (typeof ChartDataLabels !== 'undefined') {
        Chart.register(ChartDataLabels);
        Chart.defaults.plugins.datalabels.anchor = 'end';
        Chart.defaults.plugins.datalabels.align = 'top';
        Chart.defaults.plugins.datalabels.formatter = (value, context) => value;
        Chart.defaults.plugins.datalabels.color = '#e0e0e0';
        Chart.defaults.plugins.datalabels.font = { weight: 'bold', size: 10 };
        console.log('ChartDataLabels plugin registered.');
    } else {
        console.warn('ChartDataLabels plugin not found.');
    }

    const initCharts = () => {
        console.log('initCharts called. hasData:', hasData);
        if (!hasData) {
            console.log('No data to display charts. Calling showFinalContent and setupPagination.');
            showFinalContent();
            setupPagination();
            return;
        }

        try {
            console.log('Attempting to parse chart data...');
            const emotionCountsRaw = statsDataContainer.dataset.emotionCounts;
            const weeklyCountsRaw = statsDataContainer.dataset.weeklyCounts;
            console.log('Raw emotionCounts:', emotionCountsRaw);
            console.log('Raw weeklyCounts:', weeklyCountsRaw);

            if (!emotionCountsRaw || !weeklyCountsRaw) {
                console.error('Chart data attributes (emotionCounts or weeklyCounts) are missing or empty strings.');
                showFinalContent();
                setupPagination();
                return;
            }
            if (emotionCountsRaw === "[]" && weeklyCountsRaw === "[]") {
                 console.log('Data attributes exist but represent empty arrays. No charts to draw.');
                 // showFinalContent and setupPagination will be called at the end of try block
            }

            const emotionData = JSON.parse(emotionCountsRaw);
            const weeklyData = JSON.parse(weeklyCountsRaw);
            console.log('Parsed emotionData:', emotionData);
            console.log('Parsed weeklyData:', weeklyData);

            const emotionLabels = emotionData.map(e => `${e.emoji} ${e.name}`);
            const emotionValues = emotionData.map(e => e.count);
            const emotionColors = emotionData.map(e => e.color);

            const weeklyLabels = weeklyData.map(w => w.week);
            const weeklyValues = weeklyData.map(w => w.count);

            // --- Initialize Bar Chart ---
            const barCanvas = document.getElementById('emotionBarChart');
            console.log('Bar chart canvas element:', barCanvas);
            if (barCanvas && emotionData.length > 0) {
                const barCtx = barCanvas.getContext('2d');
                if (barCtx) {
                    console.log('Initializing Bar Chart...');
                    new Chart(barCtx, {
                        type: 'bar',
                        data: {
                            labels: emotionLabels,
                            datasets: [{
                                label: 'Count',
                                data: emotionValues,
                                backgroundColor: emotionColors,
                                borderColor: emotionColors.map(color => chroma(color).darken(0.5).hex()),
                                borderWidth: 1
                            }]
                        },
                        options: {
                            maintainAspectRatio: true, // Let Chart.js manage this for bar
                            plugins: {
                                legend: { display: false }, title: { display: false },
                                datalabels: { color: '#fff', anchor: 'center', align: 'center', font: { size: 11, weight: 'bold' } }
                            },
                            scales: {
                                y: { beginAtZero: true, ticks: { color: '#bdc1c6', stepSize: 1 }, grid: { color: 'rgba(255, 255, 255, 0.1)' } },
                                x: { ticks: { color: '#bdc1c6' }, grid: { display: false } }
                            }
                        }
                    });
                } else { console.error('Failed to get 2D context for bar chart.'); }
            } else { console.log('Bar chart canvas not found or no emotion data for bar chart.'); }

            // --- Initialize Pie Chart ---
            const pieCanvas = document.getElementById('emotionPieChart');
            console.log('Pie chart canvas element:', pieCanvas);
            if (pieCanvas && emotionData.length > 0) {
                const pieCtx = pieCanvas.getContext('2d');
                if (pieCtx) {
                    console.log('Initializing Pie Chart...');
                    new Chart(pieCtx, {
                        type: 'pie',
                        data: {
                            labels: emotionLabels,
                            datasets: [{
                                data: emotionValues, backgroundColor: emotionColors,
                                borderColor: '#2f3241', borderWidth: 2, hoverOffset: 8
                            }]
                        },
                        options: {
                            maintainAspectRatio: true, // Recommended for pie/doughnut
                            animation: { animateRotate: true, animateScale: true, duration: 1200 },
                            plugins: {
                                legend: { position: 'bottom', labels: { color: '#bdc1c6', boxWidth: 15, padding: 15 } },
                                title: { display: false },
                                datalabels: {
                                    formatter: (value, ctx) => {
                                        let sum = ctx.chart.data.datasets[0].data.reduce((a, b) => a + b, 0);
                                        let percentage = sum > 0 ? (value * 100 / sum).toFixed(1) + "%" : "0%";
                                        return percentage;
                                    },
                                    color: '#FFF',
                                    backgroundColor: (context) => chroma(context.dataset.backgroundColor[context.dataIndex]).darken(0.3).alpha(0.7).hex(),
                                    borderRadius: 4, padding: 4, font: { weight: 'bold', size: 10 }
                                }
                            }
                        }
                    });
                } else { console.error('Failed to get 2D context for pie chart.'); }
            } else { console.log('Pie chart canvas not found or no emotion data for pie chart.'); }

            // --- Initialize Line Chart ---
            const lineCanvas = document.getElementById('weeklyLineChart');
            console.log('Line chart canvas element:', lineCanvas);
            if (lineCanvas && weeklyData.length > 0) {
                const lineCtx = lineCanvas.getContext('2d');
                if (lineCtx) {
                    console.log('Initializing Line Chart...');
                    new Chart(lineCtx, {
                        type: 'line',
                        data: {
                            labels: weeklyLabels,
                            datasets: [{
                                label: 'Entries per Week', data: weeklyValues, fill: true,
                                backgroundColor: 'rgba(160, 116, 181, 0.2)', borderColor: '#A074B5',
                                tension: 0.3, pointBackgroundColor: '#A074B5', pointBorderColor: '#fff',
                                pointHoverBackgroundColor: '#fff', pointHoverBorderColor: '#A074B5',
                                pointRadius: 4, pointHoverRadius: 6
                            }]
                        },
                        options: {
                            maintainAspectRatio: true, // Let Chart.js manage this for line
                            plugins: {
                                legend: { display: false }, title: { display: false },
                                datalabels: { align: 'top', offset: 8, color: '#e0e0e0' }
                            },
                            scales: {
                                y: { beginAtZero: true, ticks: { color: '#bdc1c6', stepSize: 1 }, grid: { color: 'rgba(255, 255, 255, 0.1)' } },
                                x: { ticks: { color: '#bdc1c6' }, grid: { color: 'rgba(255, 255, 255, 0.05)' } }
                            }
                        }
                    });
                } else { console.error('Failed to get 2D context for line chart.'); }
            } else { console.log('Line chart canvas not found or no weekly data for line chart.'); }

            console.log('Chart initializations attempted.');
            showFinalContent();
            setupPagination();

        } catch (e) {
            console.error("CRITICAL: Error during initCharts:", e);
            showFinalContent();
            setupPagination();
        }
    };

    const showFinalContent = () => {
        console.log('showFinalContent called.');
        if (loadingIndicator) {
            console.log('Hiding loading indicator.');
            loadingIndicator.style.display = 'none';
        }
        if (mainContent) {
            console.log('Processing mainContent: adding is-loaded.');
            mainContent.classList.add('is-loaded');
            console.log('mainContent classes after update:', mainContent.classList.toString());
        } else {
            console.error('CRITICAL: mainContent element not found in showFinalContent');
        }
        if (overallStatsContainer) {
            console.log('Removing is-loading from overallStatsContainer.');
            overallStatsContainer.classList.remove('is-loading');
            console.log('overallStatsContainer classes after update:', overallStatsContainer.classList.toString());
        } else {
            console.error('CRITICAL: overallStatsContainer element not found in showFinalContent');
        }
    };

    const setupPagination = () => {
        console.log('setupPagination called.');
        const pages = document.querySelectorAll('.stats-page');
        const prevButton = document.getElementById('stats-prev-page');
        const nextButton = document.getElementById('stats-next-page');
        const pageIndicator = document.getElementById('stats-page-indicator');
        const paginationControls = document.querySelector('.stats-custom-pagination');

        if (!pages.length || !paginationControls) {
             console.log('No pages found or paginationControls missing. Hiding pagination.');
            if(paginationControls) paginationControls.style.display = 'none';
            if (pages.length > 0 && !hasData) pages[0].classList.add('active-stats-page');
            return;
        }

        if (pages.length <= 1 || !hasData) {
            console.log('Not enough pages or no data. Hiding pagination.');
            paginationControls.style.display = 'none';
            if (pages.length > 0) { // Ensure first page (e.g., "no-stats" message) is shown
                pages[0].classList.add('active-stats-page');
            }
            return;
        }

        console.log(`Found ${pages.length} pages. Setting up pagination controls.`);
        paginationControls.style.display = 'flex';
        let currentPageIndex = 0;

        const updatePageDisplay = () => {
            pages.forEach((page, index) => {
                page.classList.toggle('active-stats-page', index === currentPageIndex);
            });
            if (prevButton) prevButton.disabled = currentPageIndex === 0;
            if (nextButton) nextButton.disabled = currentPageIndex === pages.length - 1;
            if (pageIndicator) pageIndicator.textContent = `Page ${currentPageIndex + 1} of ${pages.length}`;
        };

        if (prevButton) {
            prevButton.addEventListener('click', () => {
                if (currentPageIndex > 0) {
                    currentPageIndex--;
                    updatePageDisplay();
                }
            });
        }

        if (nextButton) {
            nextButton.addEventListener('click', () => {
                if (currentPageIndex < pages.length - 1) {
                    currentPageIndex++;
                    updatePageDisplay();
                }
            });
        }
        updatePageDisplay(); // Initial setup
    };

    console.log('Initial call to initCharts logic...');
    // Delay chart initialization slightly to ensure containers are fully rendered
    setTimeout(initCharts, 150); // Slightly increased delay

});

// Fallback for chroma.js
if (typeof chroma === 'undefined') {
    window.chroma = (color) => ({
        darken: () => ({ hex: () => color }),
        alpha: () => ({ hex: () => color })
    });
    console.warn('chroma.js not found. Advanced color manipulation for charts will be basic.');
}