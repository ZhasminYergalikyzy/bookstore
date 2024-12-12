document.addEventListener("DOMContentLoaded", () => {
    // Кнопки "Купить"
    const buyButtons = document.querySelectorAll(".card button");

    buyButtons.forEach(button => {
        button.addEventListener("click", () => {
            // Получаем ID книги (если оно задано в data-атрибуте)
            const bookId = button.getAttribute("data-book-id");

            // Отправляем запрос на сервер
            fetch("http://localhost:8080/buy", {
                method: "POST",
                headers: {
                    "Content-Type": "application/json"
                },
                body: JSON.stringify({
                    book_id: bookId
                })
            })
            .then(response => response.json())
            .then(data => {
                if (data.status === "success") {
                    alert("Книга добавлена в корзину!");
                } else {
                    alert("Произошла ошибка!");
                }
            })
            .catch(error => {
                console.error("Ошибка при отправке данных:", error);
                alert("Произошла ошибка при отправке запроса.");
            });
        });
    });

    // Скролл категорий
    const categories = document.querySelector(".category-list");
    categories.addEventListener("wheel", (event) => {
        event.preventDefault();
        categories.scrollLeft += event.deltaY;
    });
});


document.addEventListener("DOMContentLoaded", () => {
    const items = document.querySelectorAll(".carousel-item");
    const nextButton = document.querySelector(".next");
    const prevButton = document.querySelector(".prev");
    let currentIndex = 0;

    function showSlide(index) {
        items.forEach((item, i) => {
            item.classList.toggle("active", i === index);
        });
    }

    nextButton.addEventListener("click", () => {
        currentIndex = (currentIndex + 1) % items.length;
        showSlide(currentIndex);
    });

    prevButton.addEventListener("click", () => {
        currentIndex = (currentIndex - 1 + items.length) % items.length;
        showSlide(currentIndex);
    });

    // Автоматическая смена слайдов
    // setInterval(() => {
    //     currentIndex = (currentIndex + 1) % items.length;
    //     showSlide(currentIndex);
    // }, 10000); // смена каждые 10 секунд
});
